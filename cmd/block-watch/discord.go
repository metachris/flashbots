package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/metachris/flashbots/blockwatch"
)

func SendBlockErrorToDiscord(b *blockwatch.Block) (success bool) {
	url := os.Getenv("DISCORD_WEBHOOK")
	if len(url) == 0 {
		log.Println("error no DISCORD_WEBHOOK")
		return false
	}

	msg := fmt.Sprintf("block %d - miner: %s, bundles: %d\n", b.Number, b.Miner, len(b.Bundles))
	for _, err := range b.Errors {
		msg += err
	}
	msg += "\n"
	for _, bundle := range b.Bundles {
		msg += fmt.Sprintf("- bundle %d: tx=%d, gasUsed=%d \t coinbase_transfer: %18v, total_miner_reward: %18v \t coinbase/gasused: %12v, reward/gasused: %12v \n", bundle.Index, len(bundle.Transactions), bundle.TotalGasUsed, bundle.TotalCoinbaseTransfer, bundle.TotalMinerReward, bundle.CoinbaseDivGasUsed, bundle.RewardDivGasUsed)
	}
	discordPayload := DiscordWebhookPayload{Content: "```" + msg + "```"}
	payloadBytes, err := json.Marshal(discordPayload)
	if err != nil {
		log.Println(err)
		return false
	}

	res, err := http.Post(url, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		log.Println(2, err)
		return false
	}

	defer res.Body.Close()
	fmt.Println("response Status:", res.Status)
	return true
}
