package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type FlashbotsBlock struct {
	BlockNumber       int64  `json:"block_number"`
	Miner             string `json:"miner"`
	MinerReward       string `json:"miner_reward"`
	CoinbaseTransfers string `json:"coinbase_transfers"`

	GasUsed      int64                  `json:"gas_used"`
	GasPrice     string                 `json:"gas_price"`
	Transactions []FlashbotsTransaction `json:"transactions"`
}

// HasTx returns true if the transaction hash is included in any of the blocks of the API response
func (b FlashbotsBlock) HasTx(hash string) bool {
	for _, tx := range b.Transactions {
		if tx.Hash == hash {
			return true
		}
	}
	return false
}

type GetBlocksOptions struct {
	BlockNumber int64
	Miner       string
	From        string
	Before      int64
	Limit       int64
}

func (b GetBlocksOptions) ToUriQuery() string {
	args := []string{}
	if b.BlockNumber > 0 {
		args = append(args, fmt.Sprintf("block_number=%d", b.BlockNumber))
	}
	if b.Miner != "" {
		args = append(args, fmt.Sprintf("miner=%s", b.Miner))
	}
	if b.From != "" {
		args = append(args, fmt.Sprintf("from=%s", b.From))
	}
	if b.Before > 0 {
		args = append(args, fmt.Sprintf("before=%d", b.Before))
	}
	if b.Limit > 0 {
		args = append(args, fmt.Sprintf("limit=%d", b.Limit))
	}

	s := strings.Join(args, "&")
	if len(s) > 0 {
		s = "?" + s
	}

	return s
}

type GetBlocksResponse struct {
	LatestBlockNumber int64            `json:"latest_block_number"`
	Blocks            []FlashbotsBlock `json:"blocks"`
}

// GetTxMap returns a map of all transactions, indexed by hash
func (r *GetBlocksResponse) GetTxMap() map[string]FlashbotsTransaction {
	res := make(map[string]FlashbotsTransaction)
	for _, b := range r.Blocks {
		for _, t := range b.Transactions {
			res[t.Hash] = t
		}
	}
	return res
}

// HasTx returns true if the transaction hash is included in any of the blocks of the API response
func (r *GetBlocksResponse) HasTx(hash string) bool {
	txMap := r.GetTxMap()
	_, exists := txMap[hash]
	return exists
}

// GetBlocks returns the 100 most recent flashbots blocks. This also contains a list of transactions that were
// part of the flashbots bundle.
// https://blocks.flashbots.net/v1/blocks
func GetBlocks(options *GetBlocksOptions) (response GetBlocksResponse, err error) {
	url := "https://blocks.flashbots.net/v1/blocks"
	if options != nil {
		url = url + options.ToUriQuery()
	}

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
