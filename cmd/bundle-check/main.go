// MEV bundles MUST be sorted by their bundle adjusted gas price first and then one by one added
// to the block as long as there is any gas left in the block and the number of bundles added
// is less or equal the MaxMergedBundles parameter.
// https://docs.flashbots.net/flashbots-core/miners/mev-geth-spec/v02/#block-construction
package main

import (
	"fmt"
	"log"
	"math/big"
	"os"
	"sort"

	"github.com/metachris/flashbots/api"
	"github.com/metachris/go-ethutils/utils"
)

type Bundle struct {
	Transactions          []api.FlashbotsTransaction
	TotalMinerReward      *big.Int
	TotalCoinbaseTransfer *big.Int
	TotalGasUsed          *big.Int
	EffectiveGasPrice     *big.Int
}

func NewBundle() *Bundle {
	return &Bundle{
		TotalMinerReward:      new(big.Int),
		TotalCoinbaseTransfer: new(big.Int),
		TotalGasUsed:          new(big.Int),
		EffectiveGasPrice:     new(big.Int),
	}
}

// type Block struct {
// 	Bundles []*Bundle
// }

// func (b *Block) AddBundle(bundle *Bundle) {
// 	b.Bundles = append(b.Bundles, bundle)
// }

func main() {
	log.SetOutput(os.Stdout)

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
		checkBlock(b)
	}
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

		txCoinbaseTransfer := new(big.Int)
		txCoinbaseTransfer.SetString(tx.CoinbaseTransfer, 10)

		txGasUsed := big.NewInt(tx.GasUsed)

		bundle.TotalMinerReward = new(big.Int).Add(bundle.TotalMinerReward, txMinerReward)
		bundle.TotalCoinbaseTransfer = new(big.Int).Add(bundle.TotalCoinbaseTransfer, txCoinbaseTransfer)
		bundle.TotalGasUsed = new(big.Int).Add(bundle.TotalGasUsed, txGasUsed)

		bundle.EffectiveGasPrice = new(big.Int).Div(bundle.TotalCoinbaseTransfer, bundle.TotalGasUsed)
	}

	numBundles := len(bundles)
	// b := Block{}
	// for _, bundle := range bundles {
	// 	b.AddBundle(bundle)
	// }

	blockHasError := false // true only after error

	// Q1: do all bundles exists or are there gaps?
	for i := 0; i < numBundles; i++ {
		if bundles[int64(i)] == nil {
			msg := fmt.Sprintf("- error: missing bundle # %d in block %d", i, block.BlockNumber)
			utils.ColorPrintf(utils.WarningColor, msg)
			blockHasError = true
		}
	}

	// Q2: are the bundles in the correct order?
	lastRefVal := new(big.Int)
	for i := 0; i < numBundles; i++ {
		bundle := bundles[int64(i)]
		// if not first bundle, and value larger than from last bundle, print the error
		if lastRefVal.Int64() != 0 && bundle.EffectiveGasPrice.Cmp(lastRefVal) == 1 {
			if !blockHasError {
				// first error - print the block
				fmt.Printf("block %d: bundles: %d, tx: %-5d\n", block.BlockNumber, numBundles, len(block.Transactions))
			}

			// print the error
			msg := fmt.Sprintf("- order error: bundle %d pays more but comes after lower effective-gas-price\n", i)
			utils.ColorPrintf(utils.WarningColor, msg)
			blockHasError = true
		}

		lastRefVal = bundle.EffectiveGasPrice
	}

	// Print all bundles if this block has an error
	if blockHasError {
		for i := 0; i < numBundles; i++ {
			bundle := bundles[int64(i)]
			fmt.Printf("- bundle %d: tx=%d \t total_miner_reward: %18v \t effective_gasprice: %14v \n", i, len(bundle.Transactions), bundle.TotalMinerReward, bundle.EffectiveGasPrice)
		}

		fmt.Println("")
	}
}
