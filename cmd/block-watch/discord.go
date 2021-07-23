// Discord webhook helpers
// https://discord.com/developers/docs/resources/webhook#execute-webhook
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

type DiscordWebhookPayload struct {
	Content string `json:"content"`
}

var discordUrl string = os.Getenv("DISCORD_WEBHOOK")

// SendToDiscord splits one message into multiple if necessary (max size is 2k characters)
func SendToDiscord(msg string) error {
	if msg == "" {
		return nil
	}

	for {
		if len(msg) < 2000 {
			return _SendToDiscord(msg)
		}

		// Extract 2k of message and send those
		smallMsg := ""
		if strings.Contains(msg, "```") {
			smallMsg = msg[0:1994] + "...```"
			msg = "```..." + msg[1994:]
		} else {
			smallMsg = msg[0:1997] + "..."
			msg = "..." + msg[1997:]
		}

		err := _SendToDiscord(smallMsg)
		if err != nil {
			return err
		}
	}
}

// _SendToDiscord sends to discord without any error checks
func _SendToDiscord(msg string) error {
	if len(discordUrl) == 0 {
		return errors.New("no DISCORD_WEBHOOK env variable found")
	}

	discordPayload := DiscordWebhookPayload{Content: msg}
	payloadBytes, err := json.Marshal(discordPayload)
	if err != nil {
		return err
	}

	res, err := http.Post(discordUrl, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return err
	}

	defer res.Body.Close()
	log.Println("Discord response status:", res.Status)

	if res.StatusCode >= 300 {
		bodyBytes, _ := ioutil.ReadAll(res.Body)
		bodyString := string(bodyBytes)
		log.Println(bodyString)
	}
	return nil
}
