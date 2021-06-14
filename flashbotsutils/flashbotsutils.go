package flashbotsutils

import (
	"errors"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/metachris/flashbots-failed-tx/flashbotsapi"
)

var (
	ErrFlashbotsApiDoesntHaveThatBlockYet = errors.New("flashbots API latest height < requested block height")
)

// IsFlashbotsTx is a utility for confirming if a specific transactions is actually a Flashbots one
func IsFlashbotsTx(block *types.Block, tx *types.Transaction) (isFlashbotsTx bool, response flashbotsapi.GetBlocksResponse, err error) {
	opts := flashbotsapi.GetBlocksOptions{BlockNumber: block.Number().Int64()}
	flashbotsResponse, err := flashbotsapi.GetBlocks(&opts)
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
