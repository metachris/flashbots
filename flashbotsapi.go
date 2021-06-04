package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type FlashbotsBlockResponseTransaction struct {
	Hash             string `json:"transaction_hash"`
	TxIndex          int64  `json:"tx_index"`
	BundleIndex      int64  `json:"bundle_index"`
	BlockNumber      int64  `json:"block_number"`
	EoaAddress       string `json:"eoa_address"`
	ToAddress        string `json:"to_address"`
	GasUsed          int64  `json:"gas_used"`
	GasPrice         string `json:"gas_price"`
	CoinbaseTransfer string `json:"coinbase_transfer"`
	TotalMinerReward string `json:"total_miner_reward"`
}

type FlashbotsBlockResponseBlock struct {
	BlockNumber  int64                               `json:"block_number"`
	GasUsed      int64                               `json:"gas_used"`
	GasPrice     string                              `json:"gas_price"`
	Transactions []FlashbotsBlockResponseTransaction `json:"transactions"`
}

type FlashbotsBlockResponse struct {
	LatestBlockNumber int64                         `json:"latest_block_number"`
	Blocks            []FlashbotsBlockResponseBlock `json:"blocks"`
}

func (r *FlashbotsBlockResponse) GetTxMap() map[string]FlashbotsBlockResponseTransaction {
	res := make(map[string]FlashbotsBlockResponseTransaction)
	for _, b := range r.Blocks {
		for _, t := range b.Transactions {
			res[t.Hash] = t
		}
	}
	return res
}

// https://blocks.flashbots.net/v1/blocks
func GetFlashbotsBlock(blockNumber int64) (response FlashbotsBlockResponse, err error) {
	url := fmt.Sprintf("https://blocks.flashbots.net/v1/blocks?block_number=%d", blockNumber)
	resp, err := http.Get(url)
	if err != nil {
		return response, err
	}

	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return response, err
	}

	return response, nil
}
