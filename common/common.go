package common

import (
	"math/big"
	"sort"

	"github.com/metachris/flashbots/api"
)

type Bundle struct {
	Index                 int64
	Transactions          []api.FlashbotsTransaction
	TotalMinerReward      *big.Int
	TotalCoinbaseTransfer *big.Int
	TotalGasUsed          *big.Int

	CoinbaseDivGasUsed *big.Int
	RewardDivGasUsed   *big.Int
}

func NewBundle() *Bundle {
	return &Bundle{
		TotalMinerReward:      new(big.Int),
		TotalCoinbaseTransfer: new(big.Int),
		TotalGasUsed:          new(big.Int),
		CoinbaseDivGasUsed:    new(big.Int),
		RewardDivGasUsed:      new(big.Int),
	}
}

type Block struct {
	Number  int64
	Miner   string
	Bundles []*Bundle

	Errors []string
}

func (b *Block) AddBundle(bundle *Bundle) {
	b.Bundles = append(b.Bundles, bundle)

	// Bring bundles into order
	sort.SliceStable(b.Bundles, func(i, j int) bool {
		return b.Bundles[i].Index < b.Bundles[j].Index
	})
}

func (b *Block) HasErrors() bool {
	return len(b.Errors) > 0
}
