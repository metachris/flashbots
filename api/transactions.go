package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type GetTransactionsOptions struct {
	Before int64 // Filter transactions to before this block number (exclusive, does not include this block number). Default value: latest
	Limit  int64 // Number of transactions that are returned
}

func (opts GetTransactionsOptions) ToUriQuery() string {
	args := []string{}
	if opts.Before > 0 {
		args = append(args, fmt.Sprintf("before=%d", opts.Before))
	}
	if opts.Limit > 0 {
		args = append(args, fmt.Sprintf("limit=%d", opts.Limit))
	}

	s := strings.Join(args, "&")
	if len(s) > 0 {
		s = "?" + s
	}

	return s
}

type TransactionsResponse struct {
	LatestBlockNumber int64                  `json:"latest_block_number"`
	Transactions      []FlashbotsTransaction `json:"transactions"`
}

// GetTransactions returns the 100 most recent flashbots transactions. Use the before query param to
// filter to transactions before a given block number.
// https://blocks.flashbots.net/#api-Flashbots-GetV1Transactions
func GetTransactions(options *GetTransactionsOptions) (response TransactionsResponse, err error) {
	url := "https://blocks.flashbots.net/v1/transactions"
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
