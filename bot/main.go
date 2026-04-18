package main

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/slack-go/slack"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	telegramToken := optionalEnv("TELEGRAM_TOKEN")
	telegramChatIDs := optionalEnv("ALLOWED_CHAT_ID")

	slackBotToken := optionalEnv("SLACK_BOT_TOKEN")
	slackAppToken := optionalEnv("SLACK_APP_TOKEN")
	slackChannelIDs := optionalEnv("SLACK_CHANNEL_ID")

	telegramEnabled := telegramToken != "" && telegramChatIDs != ""
	slackEnabled := slackBotToken != "" && slackAppToken != ""

	if !telegramEnabled {
		log.Printf("telegram: TELEGRAM_TOKEN or ALLOWED_CHAT_ID not set, skipping telegram")
	}
	if !slackEnabled {
		log.Printf("slack: SLACK_BOT_TOKEN or SLACK_APP_TOKEN not set, skipping slack")
	}

	var telegramBot *tgbotapi.BotAPI
	var telegramChats []int64
	if telegramEnabled {
		var err error
		telegramBot, err = tgbotapi.NewBotAPI(telegramToken)
		if err != nil {
			log.Fatalf("telegram init: %v", err)
		}
		telegramChats = parseChatIDList(telegramChatIDs)
	}

	var slackAPI *slack.Client
	var slackChannels []string
	if slackEnabled {
		slackAPI = slack.New(slackBotToken, slack.OptionAppLevelToken(slackAppToken))
		slackChannels = parseStringList(slackChannelIDs)
	}

	cronCallback := func(text string) {
		log.Printf("cron/background result: %s", text)
		if telegramBot != nil {
			telegramBroadcast(telegramBot, telegramChats, text)
		}
		if slackAPI != nil {
			slackBroadcast(slackAPI, slackChannels, text)
		}
	}

	state := &agentState{
		cronCb: cronCallback,
	}

	var err error
	state.claude, err = startClaude(cronCallback)
	if err != nil {
		log.Fatalf("failed to start initial claude process: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("received %s, shutting down", sig)
		state.mu.Lock()
		if state.claude != nil {
			state.claude.stop()
		}
		state.mu.Unlock()
		os.Exit(0)
	}()

	var wg sync.WaitGroup

	if telegramEnabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed := parseAllowedIDs(telegramChatIDs)
			startTelegram(state, telegramToken, allowed)
		}()
	}

	if slackEnabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowedChannels := parseStringSet(slackChannelIDs)
			startSlack(state, slackBotToken, slackAppToken, allowedChannels)
		}()
	}

	wg.Wait()
}
