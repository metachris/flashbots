// MEV bundles MUST be sorted by their bundle adjusted gas price first and then one by one added
// to the block as long as there is any gas left in the block and the number of bundles added
// is less or equal the MaxMergedBundles parameter.
// https://docs.flashbots.net/flashbots-core/miners/mev-geth-spec/v02/#block-construction
package blockwatch

import (
	"fmt"
	"log"
	"math/big"
	"sort"

	"github.com/metachris/flashbots/api"
	"github.com/metachris/flashbots/common"
	"github.com/metachris/go-ethutils/utils"
)

func CheckRecent() {
	blocks, err := api.GetBlocks(&api.GetBlocksOptions{Limit: 1000})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%d blocks\n", len(blocks.Blocks))

	// Sort by blockheight, to iterate in ascending order
	sort.SliceStable(blocks.Blocks, func(i, j int) bool {
		return blocks.Blocks[i].BlockNumber > blocks.Blocks[j].BlockNumber
	})

	// Check each block
	for _, b := range blocks.Blocks {
		CheckBlock(b)
	}
}

type Block struct {
	Number  int64
	Bundles []*common.Bundle

	Errors []string
}

func (b *Block) AddBundle(bundle *common.Bundle) {
	b.Bundles = append(b.Bundles, bundle)

	// Bring bundles into order
	sort.SliceStable(b.Bundles, func(i, j int) bool {
		return b.Bundles[i].Index < b.Bundles[j].Index
	})
}

func (b *Block) HasErrors() bool {
	return len(b.Errors) > 0
}

func PrintBlock(b *Block) {
	// Print block info
	fmt.Printf("block %d: bundles: %d\n", b.Number, len(b.Bundles))

	// Print errors
	for _, err := range b.Errors {
		utils.ColorPrintf(utils.WarningColor, err)
	}

	// Print bundles
	for _, bundle := range b.Bundles {
		fmt.Printf("- bundle %d: tx=%d, gasUsed=%d \t coinbase_transfer: %18v, total_miner_reward: %18v \t coinbase/gasused: %12v, reward/gasused: %12v \n", bundle.Index, len(bundle.Transactions), bundle.TotalGasUsed, bundle.TotalCoinbaseTransfer, bundle.TotalMinerReward, bundle.CoinbaseDivGasUsed, bundle.RewardDivGasUsed)
	}
}

func CheckBlock(block api.FlashbotsBlock) *Block {
	// Create the bundles from this block
	bundles := make(map[int64]*common.Bundle)
	for _, tx := range block.Transactions {
		bundleIndex := tx.BundleIndex
		bundle, exists := bundles[bundleIndex]
		if !exists {
			bundle = common.NewBundle()
			bundle.Index = bundleIndex
			bundles[bundleIndex] = bundle
		}

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

	numBundles := len(bundles)

	b := Block{Number: block.BlockNumber}
	for _, bundle := range bundles {
		b.AddBundle(bundle)
	}

	// Q1: do all bundles exists or are there gaps?
	for i := 0; i < numBundles; i++ {
		if bundles[int64(i)] == nil {
			msg := fmt.Sprintf("- error: missing bundle # %d in block %d", i, block.BlockNumber)
			b.Errors = append(b.Errors, msg)
		}
	}

	// Q2: are the bundles in the correct order?
	lastCoinbaseDivGasused := new(big.Int)
	lastRewardDivGasused := new(big.Int)
	for i := 0; i < numBundles; i++ {
		bundle := bundles[int64(i)]
		// if not first bundle, and value larger than from last bundle, print the error
		if lastCoinbaseDivGasused.Int64() != 0 && bundle.CoinbaseDivGasUsed.Cmp(lastCoinbaseDivGasused) == 1 && bundle.RewardDivGasUsed.Cmp(lastRewardDivGasused) == 1 {
			msg := fmt.Sprintf("- order error: bundle %d pays more but comes after lower price\n", bundle.Index)
			b.Errors = append(b.Errors, msg)
		}

		lastCoinbaseDivGasused = bundle.CoinbaseDivGasUsed
		lastRewardDivGasused = bundle.RewardDivGasUsed
	}

	return &b
}
