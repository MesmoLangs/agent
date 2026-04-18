package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	telegramMaxLen = 4000
	typingInterval = 4 * time.Second
)

const (
	cmdStart  = "/start"
	cmdNew    = "/new"
	cmdReset  = "/reset"
	cmdClear  = "/clear"
	cmdStatus = "/status"
	cmdHelp   = "/help"
)

const telegramHelpText = `/new — Reset session and start fresh
/status — Show bot and session status
/help — Show this message

Send any other text to chat with Claude.
Claude runs as a persistent process — full conversation memory and state are maintained until reset.`

func startTelegram(state *agentState, token string, allowed map[int64]bool) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatalf("telegram init: %v", err)
	}
	log.Printf("telegram: authorized as @%s", bot.Self.UserName)

	botCommands := tgbotapi.NewSetMyCommands(
		tgbotapi.BotCommand{Command: "new", Description: "Reset session and start fresh"},
		tgbotapi.BotCommand{Command: "status", Description: "Show bot and session status"},
		tgbotapi.BotCommand{Command: "help", Description: "List available commands"},
	)
	if _, err := bot.Request(botCommands); err != nil {
		log.Printf("telegram: failed to set bot commands: %v", err)
	}

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
				telegramSendReply(bot, msg, "Claude Agent ready. Send any message.")
				return
			case cmdNew, cmdReset, cmdClear:
				state.mu.Lock()
				resetErr := state.resetSession()
				state.mu.Unlock()
				if resetErr != nil {
					telegramSendReply(bot, msg, fmt.Sprintf("Failed to restart Claude: %v", resetErr))
					return
				}
				telegramSendReply(bot, msg, "Session reset. Claude restarted with fresh context.")
				return
			case cmdStatus:
				state.mu.Lock()
				active := state.isActive()
				task := state.currentTask
				state.mu.Unlock()
				statusMsg := fmt.Sprintf("Running. Claude process active: %v", active)
				if task != "" {
					statusMsg += fmt.Sprintf("\nCurrently working on: %s", task)
				}
				telegramSendReply(bot, msg, statusMsg)
				return
			case cmdHelp:
				telegramSendReply(bot, msg, telegramHelpText)
				return
			}

			log.Printf("telegram: processing [chat %d]: %.80s", msg.Chat.ID, text)

			if !state.mu.TryLock() {
				busyTask := state.currentTask
				telegramSendReply(bot, msg, fmt.Sprintf("I'm currently working on: \"%s\" — please wait.", busyTask))
				return
			}

			state.currentTask = truncate(text, 80)

			typingDone := make(chan struct{})
			go telegramKeepTyping(typingDone, bot, msg.Chat.ID)

			out, sendErr := state.ensureSend(text)
			state.currentTask = ""
			state.mu.Unlock()

			close(typingDone)

			if sendErr != nil {
				telegramSendReply(bot, msg, fmt.Sprintf("Error: %v", sendErr))
				return
			}

			for _, chunk := range splitMessage(out, telegramMaxLen) {
				telegramSendReply(bot, msg, chunk)
			}

			log.Printf("telegram: responded [chat %d]: %d chars", msg.Chat.ID, len(out))
		}()
	}
}

func telegramSendReply(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, text string) {
	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	reply.ReplyToMessageID = msg.MessageID
	if _, err := bot.Send(reply); err != nil {
		log.Printf("telegram: send error [chat %d]: %v", msg.Chat.ID, err)
	}
}

func telegramKeepTyping(done chan struct{}, bot *tgbotapi.BotAPI, chatID int64) {
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

func telegramBroadcast(bot *tgbotapi.BotAPI, chatIDs []int64, text string) {
	for _, chatID := range chatIDs {
		for _, chunk := range splitMessage(text, telegramMaxLen) {
			reply := tgbotapi.NewMessage(chatID, chunk)
			if _, err := bot.Send(reply); err != nil {
				log.Printf("telegram: broadcast error [chat %d]: %v", chatID, err)
			}
		}
	}
}
