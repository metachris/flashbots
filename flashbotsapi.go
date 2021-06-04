package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/ethereum/go-ethereum/core/types"
)

type FlashbotsApiTransaction struct {
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

type FlashbotsApiBlock struct {
	BlockNumber       int64  `json:"block_number"`
	Miner             string `json:"miner"`
	MinerReward       string `json:"miner_reward"`
	CoinbaseTransfers string `json:"coinbase_transfers"`

	GasUsed      int64                     `json:"gas_used"`
	GasPrice     string                    `json:"gas_price"`
	Transactions []FlashbotsApiTransaction `json:"transactions"`
}

type FlashbotsBlockApiResponse struct {
	LatestBlockNumber int64               `json:"latest_block_number"`
	Blocks            []FlashbotsApiBlock `json:"blocks"`
}

func (r *FlashbotsBlockApiResponse) GetTxMap() map[string]FlashbotsApiTransaction {
	res := make(map[string]FlashbotsApiTransaction)
	for _, b := range r.Blocks {
		for _, t := range b.Transactions {
			res[t.Hash] = t
		}
	}
	return res
}

// https://blocks.flashbots.net/v1/blocks
func GetFlashbotsBlock(blockNumber int64) (response FlashbotsBlockApiResponse, err error) {
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

var (
	ErrFlashbotsApiDoesntHaveThatBlockYet = errors.New("flashbots API latest height < block height")
)

func IsFlashbotsTx(block *types.Block, tx *types.Transaction) (isFlashbotsTx bool, err error) {
	flashbotsResponse, err := GetFlashbotsBlock(block.Number().Int64())
	if err != nil {
		return isFlashbotsTx, err
	}

	if flashbotsResponse.LatestBlockNumber < block.Number().Int64() {
		return isFlashbotsTx, ErrFlashbotsApiDoesntHaveThatBlockYet
	}

	flashbotsTx := flashbotsResponse.GetTxMap()

	// fmt.Println("x", tx.Hash().String())
	// for k := range flashbotsTx {
	// 	fmt.Println("-", k)
	// }
	_, exists := flashbotsTx[tx.Hash().String()]
	return exists, nil
}
