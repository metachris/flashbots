package flashbotsutils

import (
	"errors"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/metachris/flashbots/api"
)

var (
	ErrFlashbotsApiDoesntHaveThatBlockYet = errors.New("flashbots API latest height < requested block height")
)

// Cache for last Flashbots API call (avoids calling multiple times per block)
type FlashbotsApiReqRes struct {
	RequestBlock int64
	Response     api.GetBlocksResponse
}

var flashbotsApiResponseCache FlashbotsApiReqRes

// IsFlashbotsTx is a utility for confirming if a specific transactions is actually a Flashbots one
func IsFlashbotsTx(block *types.Block, tx *types.Transaction) (isFlashbotsTx bool, response api.GetBlocksResponse, err error) {
	if flashbotsApiResponseCache.RequestBlock == block.Number().Int64() {
		isFlashbotsTx = flashbotsApiResponseCache.Response.HasTx(tx.Hash().String())
		return isFlashbotsTx, flashbotsApiResponseCache.Response, nil
	}

	opts := api.GetBlocksOptions{BlockNumber: block.Number().Int64()}
	flashbotsResponse, err := api.GetBlocks(&opts)
	if err != nil {
		return isFlashbotsTx, flashbotsResponse, err
	}

	if flashbotsResponse.LatestBlockNumber < block.Number().Int64() { // block is not yet processed by Flashbots
		return isFlashbotsTx, flashbotsResponse, ErrFlashbotsApiDoesntHaveThatBlockYet
	}

	flashbotsApiResponseCache.RequestBlock = block.Number().Int64()
	flashbotsApiResponseCache.Response = flashbotsResponse

	flashbotsTx := flashbotsResponse.GetTxMap()
	_, exists := flashbotsTx[tx.Hash().String()]
	return exists, flashbotsResponse, nil
}
