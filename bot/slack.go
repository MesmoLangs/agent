package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

const slackMaxLen = 3000

const slackHelpText = "`/new` — Reset session and start fresh\n" +
	"`/status` — Show bot and session status\n" +
	"`/help` — Show this message\n\n" +
	"Send any other message to chat with Claude.\n" +
	"Claude runs as a persistent process — full conversation memory and state are maintained until reset."

func startSlack(state *agentState, botToken string, appToken string, allowedChannels map[string]bool) {
	api := slack.New(botToken, slack.OptionAppLevelToken(appToken))
	client := socketmode.New(api)

	go func() {
		for evt := range client.Events {
			switch evt.Type {
			case socketmode.EventTypeEventsAPI:
				eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
				if !ok {
					continue
				}
				client.Ack(*evt.Request)
				handleSlackEvent(state, api, eventsAPIEvent, allowedChannels)

			case socketmode.EventTypeSlashCommand:
				cmd, ok := evt.Data.(slack.SlashCommand)
				if !ok {
					continue
				}
				client.Ack(*evt.Request)
				handleSlackCommand(state, api, cmd, allowedChannels)
			}
		}
	}()

	log.Printf("slack: connecting via socket mode")
	if err := client.Run(); err != nil {
		log.Printf("slack: socket mode error: %v", err)
	}
}

func handleSlackEvent(state *agentState, api *slack.Client, event slackevents.EventsAPIEvent, allowedChannels map[string]bool) {
	if event.Type != slackevents.CallbackEvent {
		return
	}

	innerEvent := event.InnerEvent
	switch ev := innerEvent.Data.(type) {
	case *slackevents.MessageEvent:
		if ev.BotID != "" || ev.SubType != "" {
			return
		}
		if len(allowedChannels) > 0 && !allowedChannels[ev.Channel] {
			return
		}

		text := strings.TrimSpace(ev.Text)
		if text == "" {
			return
		}

		go handleSlackMessage(state, api, ev.Channel, ev.TimeStamp, text)
	}
}

func handleSlackCommand(state *agentState, api *slack.Client, cmd slack.SlashCommand, allowedChannels map[string]bool) {
	if len(allowedChannels) > 0 && !allowedChannels[cmd.ChannelID] {
		return
	}

	switch cmd.Command {
	case "/new", "/reset", "/clear":
		state.mu.Lock()
		resetErr := state.resetSession()
		state.mu.Unlock()
		if resetErr != nil {
			slackPostMessage(api, cmd.ChannelID, fmt.Sprintf("Failed to restart Claude: %v", resetErr))
			return
		}
		slackPostMessage(api, cmd.ChannelID, "Session reset. Claude restarted with fresh context.")

	case "/status":
		state.mu.Lock()
		active := state.isActive()
		task := state.currentTask
		state.mu.Unlock()
		statusMsg := fmt.Sprintf("Running. Claude process active: %v", active)
		if task != "" {
			statusMsg += fmt.Sprintf("\nCurrently working on: %s", task)
		}
		slackPostMessage(api, cmd.ChannelID, statusMsg)

	case "/help":
		slackPostMessage(api, cmd.ChannelID, slackHelpText)
	}
}

func handleSlackMessage(state *agentState, api *slack.Client, channel string, threadTS string, text string) {
	log.Printf("slack: processing [channel %s]: %.80s", channel, text)

	if !state.mu.TryLock() {
		busyTask := state.currentTask
		slackPostReply(api, channel, threadTS, fmt.Sprintf("I'm currently working on: \"%s\" — please wait.", busyTask))
		return
	}

	state.currentTask = truncate(text, 80)

	out, sendErr := state.ensureSend(text)
	state.currentTask = ""
	state.mu.Unlock()

	if sendErr != nil {
		slackPostReply(api, channel, threadTS, fmt.Sprintf("Error: %v", sendErr))
		return
	}

	for _, chunk := range splitMessage(out, slackMaxLen) {
		slackPostReply(api, channel, threadTS, chunk)
	}

	log.Printf("slack: responded [channel %s]: %d chars", channel, len(out))
}

func slackPostMessage(api *slack.Client, channel string, text string) {
	_, _, err := api.PostMessage(channel, slack.MsgOptionText(text, false))
	if err != nil {
		log.Printf("slack: send error [channel %s]: %v", channel, err)
	}
}

func slackPostReply(api *slack.Client, channel string, threadTS string, text string) {
	_, _, err := api.PostMessage(channel, slack.MsgOptionText(text, false), slack.MsgOptionTS(threadTS))
	if err != nil {
		log.Printf("slack: reply error [channel %s]: %v", channel, err)
	}
}

func slackBroadcast(api *slack.Client, channels []string, text string) {
	for _, channel := range channels {
		for _, chunk := range splitMessage(text, slackMaxLen) {
			slackPostMessage(api, channel, chunk)
		}
	}
}
