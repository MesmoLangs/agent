package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	telegramMaxLen    = 4000
	claudeWorkDir     = "/workspace"
	typingInterval    = 4 * time.Second
	claudeTurnTimeout = 10 * time.Minute
)

const (
	cmdStart  = "/start"
	cmdNew    = "/new"
	cmdReset  = "/reset"
	cmdClear  = "/clear"
	cmdStatus = "/status"
	cmdHelp   = "/help"
)

const helpText = `/new — Reset session and start fresh
/status — Show bot and session status
/help — Show this message

Send any other text to chat with Claude.
Claude runs as a persistent process — full conversation memory and state are maintained until reset.`

type claudeInput struct {
	Type    string             `json:"type"`
	Message claudeInputMessage `json:"message"`
}

type claudeInputMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeOutput struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	Result  string `json:"result"`
}

type claudeProcess struct {
	cmd          *exec.Cmd
	stdin        io.WriteCloser
	sendMu       sync.Mutex
	dispatchMu   sync.Mutex
	responseCh   chan string
	onCronResult func(string)
	dead         chan struct{}
}

func startClaude(onCronResult func(string)) (*claudeProcess, error) {
	cmd := exec.Command("claude",
		"-p",
		"--verbose",
		"--input-format", "stream-json",
		"--output-format", "stream-json",
		"--dangerously-skip-permissions",
	)
	cmd.Dir = claudeWorkDir

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start claude: %w", err)
	}

	cp := &claudeProcess{
		cmd:          cmd,
		stdin:        stdin,
		onCronResult: onCronResult,
		dead:         make(chan struct{}),
	}

	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			log.Printf("claude -> %s", line)

			cp.dispatchMu.Lock()
			ch := cp.responseCh
			cb := cp.onCronResult
			cp.dispatchMu.Unlock()

			if ch != nil {
				select {
				case ch <- line:
				default:
					log.Printf("claude output dropped (response channel full)")
				}
			} else {
				var out claudeOutput
				if json.Unmarshal([]byte(line), &out) == nil && out.Type == "result" {
					result := strings.TrimSpace(out.Result)
					if result != "" && cb != nil {
						go cb(result)
					}
				}
			}
		}
		if err := scanner.Err(); err != nil {
			log.Printf("claude stdout scanner error: %v", err)
		}
		close(cp.dead)
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 256*1024), 256*1024)
		for scanner.Scan() {
			log.Printf("claude stderr: %s", scanner.Text())
		}
	}()

	log.Printf("claude process started (pid %d)", cmd.Process.Pid)
	return cp, nil
}

func (cp *claudeProcess) send(text string) (string, error) {
	cp.sendMu.Lock()
	defer cp.sendMu.Unlock()

	ch := make(chan string, 512)

	cp.dispatchMu.Lock()
	cp.responseCh = ch
	cp.dispatchMu.Unlock()

	defer func() {
		cp.dispatchMu.Lock()
		cp.responseCh = nil
		cp.dispatchMu.Unlock()
		for {
			select {
			case <-ch:
			default:
				return
			}
		}
	}()

	input := claudeInput{
		Type: "user",
		Message: claudeInputMessage{
			Role:    "user",
			Content: text,
		},
	}

	data, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("marshal input: %w", err)
	}

	log.Printf("claude <- %s", string(data))

	if _, err := cp.stdin.Write(append(data, '\n')); err != nil {
		return "", fmt.Errorf("write to claude stdin: %w", err)
	}

	timeout := time.After(claudeTurnTimeout)
	for {
		select {
		case line, ok := <-ch:
			if !ok {
				return "", fmt.Errorf("response channel closed")
			}

			var out claudeOutput
			if err := json.Unmarshal([]byte(line), &out); err != nil {
				continue
			}

			if out.Type == "result" {
				result := strings.TrimSpace(out.Result)
				if result == "" {
					result = "(empty response)"
				}
				log.Printf("claude response: %s", result)
				return result, nil
			}

		case <-cp.dead:
			return "", fmt.Errorf("claude process exited during response")

		case <-timeout:
			return "", fmt.Errorf("claude response timeout (%v)", claudeTurnTimeout)
		}
	}
}

func (cp *claudeProcess) stop() {
	cp.stdin.Close()

	done := make(chan struct{})
	go func() {
		cp.cmd.Wait()
		close(done)
	}()

	cp.cmd.Process.Signal(syscall.SIGTERM)

	select {
	case <-done:
		log.Printf("claude process stopped gracefully")
	case <-time.After(5 * time.Second):
		cp.cmd.Process.Kill()
		<-done
		log.Printf("claude process killed after timeout")
	}
}

func main() {
	token := requireEnv("TELEGRAM_TOKEN")
	rawIDs := requireEnv("ALLOWED_CHAT_ID")
	allowed := parseAllowedIDs(rawIDs)
	chatIDs := parseChatIDList(rawIDs)

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatalf("telegram init: %v", err)
	}
	log.Printf("authorized as @%s, allowed chats: %s", bot.Self.UserName, rawIDs)

	botCommands := tgbotapi.NewSetMyCommands(
		tgbotapi.BotCommand{Command: "new", Description: "Reset session and start fresh"},
		tgbotapi.BotCommand{Command: "status", Description: "Show bot and session status"},
		tgbotapi.BotCommand{Command: "help", Description: "List available commands"},
	)
	if _, err := bot.Request(botCommands); err != nil {
		log.Printf("failed to set bot commands: %v", err)
	}

	cronCallback := func(text string) {
		log.Printf("cron/background result: %s", text)
		for _, chatID := range chatIDs {
			for _, chunk := range splitMessage(text, telegramMaxLen) {
				reply := tgbotapi.NewMessage(chatID, chunk)
				if _, err := bot.Send(reply); err != nil {
					log.Printf("cron send error [chat %d]: %v", chatID, err)
				}
			}
		}
	}

	var (
		mu     sync.Mutex
		claude *claudeProcess
	)

	claude, err = startClaude(cronCallback)
	if err != nil {
		log.Fatalf("failed to start initial claude process: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("received %s, shutting down", sig)
		mu.Lock()
		if claude != nil {
			claude.stop()
		}
		mu.Unlock()
		os.Exit(0)
	}()

	cfg := tgbotapi.NewUpdate(0)
	cfg.Timeout = 60

	for update := range bot.GetUpdatesChan(cfg) {
		if update.Message == nil || !allowed[update.Message.Chat.ID] {
			continue
		}
		msg := update.Message
		text := strings.TrimSpace(msg.Text)
		if text == "" {
			continue
		}

		go func() {
			switch text {
			case cmdStart:
				sendReply(bot, msg, "Claude Agent ready. Send any message.")
				return
			case cmdNew, cmdReset, cmdClear:
				mu.Lock()
				if claude != nil {
					claude.stop()
				}
				var startErr error
				claude, startErr = startClaude(cronCallback)
				mu.Unlock()
				if startErr != nil {
					sendReply(bot, msg, fmt.Sprintf("Failed to restart Claude: %v", startErr))
					return
				}
				sendReply(bot, msg, "Session reset. Claude restarted with fresh context.")
				return
			case cmdStatus:
				mu.Lock()
				active := claude != nil
				mu.Unlock()
				sendReply(bot, msg, fmt.Sprintf("Running. Claude process active: %v", active))
				return
			case cmdHelp:
				sendReply(bot, msg, helpText)
				return
			}

			log.Printf("processing [chat %d]: %.80s", msg.Chat.ID, text)

			typingDone := make(chan struct{})
			go func() {
				keepTyping(typingDone, bot, msg.Chat.ID)
			}()

			mu.Lock()
			out, sendErr := ensureSend(text, &claude, cronCallback)
			mu.Unlock()

			close(typingDone)

			if sendErr != nil {
				sendReply(bot, msg, fmt.Sprintf("Error: %v", sendErr))
				return
			}

			for _, chunk := range splitMessage(out, telegramMaxLen) {
				sendReply(bot, msg, chunk)
			}

			log.Printf("responded [chat %d]: %d chars", msg.Chat.ID, len(out))
		}()
	}
}

func ensureSend(text string, claude **claudeProcess, cronCb func(string)) (string, error) {
	if *claude == nil {
		var err error
		*claude, err = startClaude(cronCb)
		if err != nil {
			return "", fmt.Errorf("start claude: %w", err)
		}
	}

	out, err := (*claude).send(text)
	if err == nil {
		return out, nil
	}

	log.Printf("claude send failed: %v, restarting", err)
	(*claude).stop()

	*claude, err = startClaude(cronCb)
	if err != nil {
		return "", fmt.Errorf("restart claude: %w", err)
	}

	return (*claude).send(text)
}

func requireEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("required env var %s is not set", key)
	}
	return val
}

func parseAllowedIDs(raw string) map[int64]bool {
	ids := make(map[int64]bool)
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			log.Fatalf("invalid chat ID %q: %v", s, err)
		}
		ids[id] = true
	}
	return ids
}

func parseChatIDList(raw string) []int64 {
	var ids []int64
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

func sendReply(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, text string) {
	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	reply.ReplyToMessageID = msg.MessageID
	if _, err := bot.Send(reply); err != nil {
		log.Printf("send error [chat %d]: %v", msg.Chat.ID, err)
	}
}

func keepTyping(done chan struct{}, bot *tgbotapi.BotAPI, chatID int64) {
	for {
		action := tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping)
		bot.Send(action)
		select {
		case <-done:
			return
		case <-time.After(typingInterval):
		}
	}
}

func splitMessage(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}
	var chunks []string
	for len(text) > 0 {
		if len(text) <= maxLen {
			chunks = append(chunks, text)
			break
		}
		cut := maxLen
		if idx := strings.LastIndex(text[:cut], "\n"); idx > 0 {
			cut = idx + 1
		}
		chunks = append(chunks, strings.TrimRight(text[:cut], "\n"))
		text = text[cut:]
	}
	return chunks
}
