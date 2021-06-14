// Client for [Flashbots mev-blocks API](https://blocks.flashbots.net/)
package flashbotsapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/ethereum/go-ethereum/core/types"
)

var (
	ErrFlashbotsApiDoesntHaveThatBlockYet = errors.New("flashbots API latest height < requested block height")
)

type FlashbotsBlockApiResponse struct {
	LatestBlockNumber int64               `json:"latest_block_number"`
	Blocks            []FlashbotsApiBlock `json:"blocks"`
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

func (r *FlashbotsBlockApiResponse) GetTxMap() map[string]FlashbotsApiTransaction {
	res := make(map[string]FlashbotsApiTransaction)
	for _, b := range r.Blocks {
		for _, t := range b.Transactions {
			res[t.Hash] = t
		}
	}
	return res
}

func (r *FlashbotsBlockApiResponse) IsFlashbotsTx(hash string) bool {
	txMap := r.GetTxMap()
	_, exists := txMap[hash]
	return exists
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

func IsFlashbotsTx(block *types.Block, tx *types.Transaction) (isFlashbotsTx bool, response FlashbotsBlockApiResponse, err error) {
	flashbotsResponse, err := GetFlashbotsBlock(block.Number().Int64())
	if err != nil {
		return isFlashbotsTx, response, err
	}

	if flashbotsResponse.LatestBlockNumber < block.Number().Int64() { // block is not yet processed by Flashbots
		return isFlashbotsTx, response, ErrFlashbotsApiDoesntHaveThatBlockYet
	}

	flashbotsTx := flashbotsResponse.GetTxMap()
	_, exists := flashbotsTx[tx.Hash().String()]
	return exists, response, nil
}
