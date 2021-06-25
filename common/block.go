package common

import (
	"fmt"
	"math/big"
	"sort"

	"github.com/metachris/flashbots/api"
)

type Block struct {
	Number  int64
	Miner   string
	Bundles []*Bundle

	Errors                        []string
	BiggestBundlePercentPriceDiff float32 // on order error, % difference to previous bundle
}

// AddBundle adds the bundle and sorts them by Index
func (b *Block) AddBundle(bundle *Bundle) {
	b.Bundles = append(b.Bundles, bundle)

	// Bring bundles into order
	sort.SliceStable(b.Bundles, func(i, j int) bool {
		return b.Bundles[i].Index < b.Bundles[j].Index
	})
}

func (b *Block) AddError(msg string) {
	b.Errors = append(b.Errors, msg)
}

func (b *Block) HasErrors() bool {
	return len(b.Errors) > 0
}

func NewBlockFromApiBlock(apiBlock api.FlashbotsBlock) *Block {
	block := Block{Number: apiBlock.BlockNumber, Miner: apiBlock.Miner}

	// Create the bundles
	bundles := make(map[int64]*Bundle)
	for _, tx := range apiBlock.Transactions {
		bundleIndex := tx.BundleIndex
		bundle, exists := bundles[bundleIndex]
		if !exists {
			bundle = NewBundle()
			bundle.Index = bundleIndex
			bundles[bundleIndex] = bundle
		}

		// Update bundle information
		bundle.Transactions = append(bundle.Transactions, tx)

		txMinerReward := new(big.Int)
		txMinerReward.SetString(tx.TotalMinerReward, 10)

		txCoinbaseTransfer := new(big.Int)
		txCoinbaseTransfer.SetString(tx.CoinbaseTransfer, 10)

		txGasUsed := big.NewInt(tx.GasUsed)

		bundle.TotalMinerReward = new(big.Int).Add(bundle.TotalMinerReward, txMinerReward)
		bundle.TotalCoinbaseTransfer = new(big.Int).Add(bundle.TotalCoinbaseTransfer, txCoinbaseTransfer)
		bundle.TotalGasUsed = new(big.Int).Add(bundle.TotalGasUsed, txGasUsed)

		bundle.CoinbaseDivGasUsed = new(big.Int).Div(bundle.TotalCoinbaseTransfer, bundle.TotalGasUsed)
		bundle.RewardDivGasUsed = new(big.Int).Div(bundle.TotalMinerReward, bundle.TotalGasUsed)
	}

	// Add bundles to the block
	for _, bundle := range bundles {
		block.AddBundle(bundle)
	}

	return &block
}

// Check analyzes the Flashbots bundles and adds errors when issues are found
func (block *Block) Check() {
	numBundles := len(block.Bundles)

	// Check 1: do all bundles exists or are there gaps?
	for i := 0; i < numBundles; i++ {
		if block.Bundles[int64(i)] == nil {
			block.AddError(fmt.Sprintf("- error: missing bundle # %d in block %d", i, block.Number))
		}
	}

	// Check 2: are the bundles in the correct order?
	lastCoinbaseDivGasused := new(big.Int)
	lastRewardDivGasused := new(big.Int)
	for i := 0; i < numBundles; i++ {
		bundle := block.Bundles[int64(i)]

		// if not first bundle, and value larger than from last bundle, print the error
		if lastCoinbaseDivGasused.Int64() == 0 {
			// nothing to do on the first bundle
		} else {
			percentDiff := new(big.Float).Quo(new(big.Float).SetInt(bundle.RewardDivGasUsed), new(big.Float).SetInt(lastRewardDivGasused))
			percentDiff = new(big.Float).Sub(percentDiff, big.NewFloat(1))
			percentDiff = new(big.Float).Mul(percentDiff, big.NewFloat(100))
			bundle.PercentPriceDiff = percentDiff

			if lastCoinbaseDivGasused.Int64() != 0 &&
				bundle.CoinbaseDivGasUsed.Cmp(lastCoinbaseDivGasused) == 1 &&
				bundle.RewardDivGasUsed.Cmp(lastRewardDivGasused) == 1 &&
				bundle.CoinbaseDivGasUsed.Cmp(lastRewardDivGasused) == 1 {

				msg := fmt.Sprintf("- order error: bundle %d pays %v%% more than previous bundle\n", bundle.Index, percentDiff.Text('f', 2))
				block.AddError(msg)
				bundle.IsOutOfOrder = true
				diffFloat, _ := percentDiff.Float32()
				if diffFloat > block.BiggestBundlePercentPriceDiff {
					block.BiggestBundlePercentPriceDiff = diffFloat
				}
			}
		}

		lastCoinbaseDivGasused = bundle.CoinbaseDivGasUsed
		lastRewardDivGasused = bundle.RewardDivGasUsed
	}
}
