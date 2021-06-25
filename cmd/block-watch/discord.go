// Discord webhook helpers
// https://discord.com/developers/docs/resources/webhook#execute-webhook
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/metachris/flashbots/blockcheck"
)

type DiscordWebhookPayload struct {
	Content string `json:"content"`
}

func SendBundleOrderErrorToDiscord(b *blockcheck.Block) error {
	msg := b.Sprint(false, true)
	return SendToDiscord(msg)
}

// func SendFailedTxToDiscord(tx failedtx.FailedTx) error {
// 	msg := failedtx.MsgForFailedTx(tx)
// 	return SendToDiscord(msg)
// }

func SendToDiscord(msg string) error {
	url := os.Getenv("DISCORD_WEBHOOK")
	if len(url) == 0 {
		return errors.New("no DISCORD_WEBHOOK env variable found")
	}

	discordPayload := DiscordWebhookPayload{Content: msg}
	payloadBytes, err := json.Marshal(discordPayload)
	if err != nil {
		return err
	}

	res, err := http.Post(url, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return err
	}

	defer res.Body.Close()
	fmt.Println("response Status:", res.Status)
	return nil
}
