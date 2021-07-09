package common

import (
	"math/big"

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

	PercentPriceDiff *big.Float // on order error, % difference to previous bundle

	IsOutOfOrder                bool
	IsPayingLessThanLowestTx    bool
	Is0EffectiveGasPrice        bool
	IsNegativeEffectiveGasPrice bool
}

func NewBundle() *Bundle {
	return &Bundle{
		TotalMinerReward:      new(big.Int),
		TotalCoinbaseTransfer: new(big.Int),
		TotalGasUsed:          new(big.Int),
		CoinbaseDivGasUsed:    new(big.Int),
		RewardDivGasUsed:      new(big.Int),
		PercentPriceDiff:      new(big.Float),
	}
}
