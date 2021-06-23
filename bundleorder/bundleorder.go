// Helper to check for correct bundle ordering within blocks
//
// MEV bundles MUST be sorted by their bundle adjusted gas price first and then one by one added
// to the block as long as there is any gas left in the block and the number of bundles added
// is less or equal the MaxMergedBundles parameter.
// https://docs.flashbots.net/flashbots-core/miners/mev-geth-spec/v02/#block-construction
package bundleorder

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

func SprintBlock(b *common.Block, color bool, markdown bool) (msg string) {
	// Print block info
	if markdown {
		msg = fmt.Sprintf("Block [%d](<https://etherscan.io/block/%d>) ([bundle-explorer](<https://flashbots-explorer.marto.lol/?block=%d>)) - miner: [%s][<https://etherscan.io/address/%s>]), bundles: %d\n", b.Number, b.Number, b.Number, b.Miner, b.Miner, len(b.Bundles))
	} else {
		msg = fmt.Sprintf("Block %d - miner: %s, bundles: %d\n", b.Number, b.Miner, len(b.Bundles))
	}

	// Print errors
	for _, err := range b.Errors {
		if color {
			msg += fmt.Sprintf(utils.WarningColor, err)
		} else {
			msg += err
		}
	}

	// Print bundles
	for _, bundle := range b.Bundles {
		// Build string for percent(gasprice difference to previous bundle)
		percentPart := ""
		if bundle.PercentPriceDiff.Cmp(big.NewFloat(0)) == -1 {
			percentPart = fmt.Sprintf("(%6s%%)", bundle.PercentPriceDiff.Text('f', 2))
		} else if bundle.PercentPriceDiff.Cmp(big.NewFloat(0)) == 1 {
			percentPart = fmt.Sprintf("(+%5s%%)", bundle.PercentPriceDiff.Text('f', 2))
		}

		msg += fmt.Sprintf("- bundle %d: tx: %d, gasUsed: %7d \t coinbase_transfer: %11v, total_miner_reward: %11v \t coinbase/gasused: %13v, reward/gasused: %13v %v", bundle.Index, len(bundle.Transactions), bundle.TotalGasUsed, common.BigIntToEString(bundle.TotalCoinbaseTransfer, 4), common.BigIntToEString(bundle.TotalMinerReward, 4), common.BigIntToEString(bundle.CoinbaseDivGasUsed, 4), common.BigIntToEString(bundle.RewardDivGasUsed, 4), percentPart)
		if bundle.IsOutOfOrder {
			msg += " <--- out_of_order"
		}
		msg += "\n"
	}

	return msg
}

func CheckBlock(block api.FlashbotsBlock) *common.Block {
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

	b := common.Block{Number: block.BlockNumber, Miner: block.Miner}
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
		if lastCoinbaseDivGasused.Int64() == 0 {

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
				b.Errors = append(b.Errors, msg)
				bundle.IsOutOfOrder = true
				diffFloat, _ := percentDiff.Float32()
				if diffFloat > b.BiggestBundlePercentPriceDiff {
					b.BiggestBundlePercentPriceDiff = diffFloat
				}
			}
		}

		lastCoinbaseDivGasused = bundle.CoinbaseDivGasUsed
		lastRewardDivGasused = bundle.RewardDivGasUsed
	}

	return &b
}
