package main

import (
	"fmt"
	"log"

	"github.com/metachris/flashbots/api"
)

func main() {
	// Blocks API: default
	block, err := api.GetBlocks(nil)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("latest block:", block.LatestBlockNumber)
	fmt.Println(len(block.Blocks), "blocks")
	fmt.Println("bundle_type:", block.Blocks[0].Transactions[0].BundleType)
	fmt.Println("is_flashbots:", block.Blocks[0].Transactions[0].BundleType == api.BundleTypeFlashbots)
	fmt.Println("is_rogue:", block.Blocks[0].Transactions[0].BundleType == api.BundleTypeRogue)
}
