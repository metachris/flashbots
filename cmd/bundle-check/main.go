// MEV bundles MUST be sorted by their bundle adjusted gas price first
// and then one by one added to the block as long as there is any gas
// left in the block and the number of bundles added is less or equal the
// MaxMergedBundles parameter.
// https://docs.flashbots.net/flashbots-core/miners/mev-geth-spec/v02/#block-construction
package main

import (
	"fmt"
	"log"
	"math/big"
	"os"

	"github.com/metachris/flashbots/api"
	"github.com/metachris/go-ethutils/utils"
)

func main() {
	log.SetOutput(os.Stdout)

	blocks, err := api.GetBlocks(nil)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%d blocks\n", len(blocks.Blocks))
	for _, b := range blocks.Blocks {
		checkBlock(b)
	}
	// sort block numbers to iterate in order

	// blockNumbers := make([]int, 0)
	// for _, b := range blocks.Blocks {
	// 	blockNumbers = append(blockNumbers, int(b.BlockNumber))
	// }
	// sort.Ints(blockNumbers)

	// fmt.Println(blockNumbers)
	// for _, height := range blockNumbers {
	// 	checkBlock(blocks.Blocks[height])
	// }
}

type Bundle struct {
	Transactions     []api.FlashbotsTransaction
	TotalMinerReward *big.Int
	TotalGasUsed     *big.Int
	AdjustedGasPrice *big.Int
}

func NewBundle() *Bundle {
	b := Bundle{
		TotalMinerReward: new(big.Int),
		TotalGasUsed:     new(big.Int),
		AdjustedGasPrice: new(big.Int),
	}
	return &b
}

func checkBlock(block api.FlashbotsBlock) {
	// fmt.Printf("block %d", block.BlockNumber)

	// Create the bundles from this block
	bundles := make(map[int64]*Bundle)
	for _, tx := range block.Transactions {
		bundleIndex := tx.BundleIndex
		bundle, exists := bundles[bundleIndex]
		if !exists {
			bundle = NewBundle()
			bundles[bundleIndex] = bundle
		}

		bundle.Transactions = append(bundle.Transactions, tx)

		txMinerReward := new(big.Int)
		txMinerReward.SetString(tx.TotalMinerReward, 10)

		txGasUsed := big.NewInt(tx.GasUsed)

		bundle.TotalMinerReward = new(big.Int).Add(bundle.TotalMinerReward, txMinerReward)
		bundle.TotalGasUsed = new(big.Int).Add(bundle.TotalGasUsed, txGasUsed)

		bundle.AdjustedGasPrice = new(big.Int).Div(bundle.TotalMinerReward, bundle.TotalGasUsed)
	}

	numBundles := len(bundles)
	printBundles := false // true only after error

	// Q1: do all bundles exists or are there gaps?
	for i := 0; i < numBundles; i++ {
		if bundles[int64(i)] == nil {
			msg := fmt.Sprintf("- error: missing bundle # %d in block %d", i, block.BlockNumber)
			utils.ColorPrintf(utils.WarningColor, msg)
			printBundles = true
		}
	}

	// Q2: are the bundles in the correct order?
	lastRefVal := new(big.Int)

	for i := 0; i < numBundles; i++ {
		bundle := bundles[int64(i)]
		// fmt.Printf("- bundle %d: tx=%d, reward: %30v \n", i, len(bundle.Transactions), bundle.TotalMinerReward)

		// if not first bundle, and value larger than from last bundle, print the error
		if lastRefVal.Int64() != 0 && bundle.AdjustedGasPrice.Cmp(lastRefVal) == 1 {
			if !printBundles {
				// first error - print the block
				fmt.Printf("block %d: bundles: %d, tx: %-5d\n", block.BlockNumber, numBundles, len(block.Transactions))

			}

			// print the error
			msg := fmt.Sprintf("- order error: bundle %d pays more but comes after lower effective-gas-price\n", i)
			utils.ColorPrintf(utils.WarningColor, msg)
			printBundles = true
		}

		lastRefVal = bundle.AdjustedGasPrice
	}

	// Print all bundles if this block has an error
	if printBundles {
		for i := 0; i < numBundles; i++ {
			bundle := bundles[int64(i)]
			fmt.Printf("- bundle %d: tx=%d \t total_miner_reward: %18v \t bundle_adjusted_gasprice: %12v \n", i, len(bundle.Transactions), bundle.TotalMinerReward, bundle.AdjustedGasPrice)
		}

		fmt.Println("")
	}
}
