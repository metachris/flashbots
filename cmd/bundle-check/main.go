// MEV bundles MUST be sorted by their bundle adjusted gas price first and then one by one added
// to the block as long as there is any gas left in the block and the number of bundles added
// is less or equal the MaxMergedBundles parameter.
// https://docs.flashbots.net/flashbots-core/miners/mev-geth-spec/v02/#block-construction
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/metachris/flashbots/api"
	"github.com/metachris/flashbots/blockwatch"
)

func main() {
	log.SetOutput(os.Stdout)

	blockHeightPtr := flag.Int64("block", 0, "specific block to check")
	flag.Parse()

	blocks, err := api.GetBlocks(&api.GetBlocksOptions{Limit: 10000, BlockNumber: *blockHeightPtr})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%d blocks\n", len(blocks.Blocks))

	// Sort by blockheight, to iterate in ascending order
	sort.SliceStable(blocks.Blocks, func(i, j int) bool {
		return blocks.Blocks[i].BlockNumber < blocks.Blocks[j].BlockNumber
	})

	// Check each block
	for _, block := range blocks.Blocks {
		b := blockwatch.CheckBlock(block)
		if b.HasErrors() {
			blockwatch.PrintBlock(b)
			fmt.Println("")
		}
	}
}
