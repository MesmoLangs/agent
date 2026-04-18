package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	claudeWorkDir     = "/workspace"
	claudeTurnTimeout = 10 * time.Minute
)

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

type claudeStreamEvent struct {
	Type    string                   `json:"type"`
	Subtype string                   `json:"subtype"`
	Result  string                   `json:"result"`
	Message claudeStreamEventMessage `json:"message"`
	Usage   claudeStreamUsage        `json:"usage"`
}

type claudeStreamEventMessage struct {
	Model   string                         `json:"model"`
	Content []claudeStreamEventContentItem `json:"content"`
}

type claudeStreamEventContentItem struct {
	Type      string                 `json:"type"`
	Text      string                 `json:"text"`
	Thinking  string                 `json:"thinking"`
	Name      string                 `json:"name"`
	Input     map[string]interface{} `json:"input"`
	Content   interface{}            `json:"content"`
	ToolUseID string                 `json:"tool_use_id"`
}

type claudeStreamUsage struct {
	OutputTokens int `json:"output_tokens"`
}

type claudeStreamCost struct {
	TotalCostUSD float64           `json:"total_cost_usd"`
	Usage        claudeStreamUsage `json:"usage"`
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

type agentState struct {
	mu          sync.Mutex
	claude      *claudeProcess
	cronCb      func(string)
	currentTask string
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

			for _, formatted := range formatClaudeLine(line) {
				log.Print(formatted)
			}

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

func (s *agentState) ensureSend(text string) (string, error) {
	if s.claude == nil {
		var err error
		s.claude, err = startClaude(s.cronCb)
		if err != nil {
			return "", fmt.Errorf("start claude: %w", err)
		}
	}

	out, err := s.claude.send(text)
	if err == nil {
		return out, nil
	}

	log.Printf("claude send failed: %v, restarting", err)
	s.claude.stop()

	s.claude, err = startClaude(s.cronCb)
	if err != nil {
		return "", fmt.Errorf("restart claude: %w", err)
	}

	return s.claude.send(text)
}

func (s *agentState) resetSession() error {
	if s.claude != nil {
		s.claude.stop()
	}
	var err error
	s.claude, err = startClaude(s.cronCb)
	return err
}

func (s *agentState) isActive() bool {
	return s.claude != nil
}

func formatClaudeLine(line string) []string {
	var ev claudeStreamEvent
	if json.Unmarshal([]byte(line), &ev) != nil {
		return nil
	}

	var lines []string

	switch ev.Type {
	case "system":
		if ev.Subtype == "init" {
			lines = append(lines, fmt.Sprintf("[init] session started, model=%s", ev.Message.Model))
		}

	case "assistant":
		for _, item := range ev.Message.Content {
			switch item.Type {
			case "thinking":
				lines = append(lines, fmt.Sprintf("[thinking] %s", truncate(item.Thinking, 120)))
			case "text":
				lines = append(lines, fmt.Sprintf("[text] %s", truncate(item.Text, 200)))
			case "tool_use":
				desc, _ := item.Input["description"].(string)
				cmd, _ := item.Input["command"].(string)
				filePath, _ := item.Input["file_path"].(string)
				pattern, _ := item.Input["pattern"].(string)
				switch {
				case desc != "":
					lines = append(lines, fmt.Sprintf("[tool] %s: %s", item.Name, truncate(desc, 150)))
				case cmd != "":
					lines = append(lines, fmt.Sprintf("[tool] %s: %s", item.Name, truncate(cmd, 150)))
				case filePath != "":
					lines = append(lines, fmt.Sprintf("[tool] %s: %s", item.Name, filePath))
				case pattern != "":
					lines = append(lines, fmt.Sprintf("[tool] %s: %s", item.Name, truncate(pattern, 150)))
				default:
					lines = append(lines, fmt.Sprintf("[tool] %s", item.Name))
				}
			}
		}

	case "user":
		for _, item := range ev.Message.Content {
			if item.Type == "tool_result" {
				resultStr := ""
				switch v := item.Content.(type) {
				case string:
					resultStr = v
				}
				if resultStr != "" {
					lines = append(lines, fmt.Sprintf("[tool_result] %s", truncate(resultStr, 200)))
				}
			}
		}

	case "result":
		var cost claudeStreamCost
		json.Unmarshal([]byte(line), &cost)
		lines = append(lines, fmt.Sprintf("[done] $%.4f | %s", cost.TotalCostUSD, truncate(ev.Result, 200)))
	}

	return lines
}
