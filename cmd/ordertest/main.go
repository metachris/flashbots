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
}

type Bundle struct {
	Transactions     []api.FlashbotsTransaction
	TotalMinerReward *big.Int
}

func NewBundle() *Bundle {
	b := Bundle{
		TotalMinerReward: new(big.Int),
	}
	return &b
}

func checkBlock(block api.FlashbotsBlock) {
	// fmt.Printf("block %d", block.BlockNumber)

	// Collect bundles in this block
	bundles := make(map[int64]*Bundle)
	for _, tx := range block.Transactions {
		bundleIndex := tx.BundleIndex
		bundle, exists := bundles[bundleIndex]
		if !exists {
			bundle = NewBundle()
			bundles[bundleIndex] = bundle
		}

		bundle.Transactions = append(bundle.Transactions, tx)

		mr := new(big.Int)
		mr.SetString(tx.TotalMinerReward, 10)
		bundle.TotalMinerReward = bundle.TotalMinerReward.Add(bundle.TotalMinerReward, mr)
	}

	numBundles := len(bundles)
	fmt.Printf("block %d: bundles: %d, tx: %-5d\n", block.BlockNumber, numBundles, len(block.Transactions))

	// for i, bundle := range bundles {
	// }

	// Q1: do all bundles exists or are there gaps?
	for i := 0; i < numBundles; i++ {
		if bundles[int64(i)] == nil {
			msg := fmt.Sprintf("- error: missing bundle # %d in block %d", i, block.BlockNumber)
			utils.ColorPrintf(utils.WarningColor, msg)
		}
	}

	// Q2: are the bundles in the correct order?
	lastBundleMinerReward := new(big.Int)

	hasBlockInvalidOrderedBundles := false
	for i := 0; i < numBundles; i++ {
		bundle := bundles[int64(i)]
		// fmt.Printf("- bundle %d: tx=%d, reward: %30v \n", i, len(bundle.Transactions), bundle.TotalMinerReward)

		// miner reward must be most in first bundle, and then less
		if lastBundleMinerReward.Cmp(big.NewInt(0)) == 0 { // first entry must set the top
			// lastBundleMinerReward = bundle.TotalMinerReward
		} else if bundle.TotalMinerReward.Cmp(lastBundleMinerReward) == 1 {
			msg := fmt.Sprintf("- order error: bundle %d pays more but comes after lower total_miner_reward\n", i)
			utils.ColorPrintf(utils.WarningColor, msg)
			hasBlockInvalidOrderedBundles = true
		}
		lastBundleMinerReward = bundle.TotalMinerReward
	}

	if hasBlockInvalidOrderedBundles {
		for i := 0; i < numBundles; i++ {
			bundle := bundles[int64(i)]
			fmt.Printf("- bundle %d: tx=%d, reward: %30v \n", i, len(bundle.Transactions), bundle.TotalMinerReward)
		}

		fmt.Println("")
	}
}
