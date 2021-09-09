// Scan a certain time range for uncle blocks, and collect statistics about miners
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	fbcommon "github.com/metachris/flashbots/common"
	"github.com/metachris/go-ethutils/addresslookup"
	"github.com/metachris/go-ethutils/utils"
)

type MinerStat struct {
	Coinbase         common.Address
	NumBlocks        int64
	NumUncled        int64 // how many blocks by the miner have been uncled (not part of mainchain)
	NumUnclings      int64 // how many blocks by the miner became mainchain where an alternative exists
	NumUnclingsPlus1 int64 // how many blocks did this miner mine that replaced another block, where the miner also mined the following block

	// Calculated afterwards
	NumUnclingsPlus1PercentOfBlocks float64
}

var minerStats map[common.Address]*MinerStat = make(map[common.Address]*MinerStat)
var uncles map[common.Hash]*types.Block = make(map[common.Hash]*types.Block)
var blocksTotal int

var client *ethclient.Client
var ethNodeUri *string

var AddressLookup *addresslookup.AddressLookupService

func main() {
	var err error
	log.SetOutput(os.Stdout)

	AddressLookup = addresslookup.NewAddressLookupService(nil)
	err = AddressLookup.AddAllAddresses()
	utils.Perror(err)

	ethNodeUri = flag.String("eth", os.Getenv("xx"), "geth node URI")
	startDate := flag.String("start", "", "time offset")
	endDate := flag.String("end", "", "time offset")
	// endDate := flag.String("end", "", "block number or time offset or date (yyyy-mm-dd) (optional)")
	flag.Parse()

	if *ethNodeUri == "" {
		log.Fatal("Missing eth node uri")
	}

	if *startDate == "" {
		log.Fatal("Missing start")
	}

	if *endDate == "" {
		log.Fatal("Missing end")
	}

	fmt.Printf("Connecting to %s ... ", *ethNodeUri)
	client, err = ethclient.Dial(*ethNodeUri)
	utils.Perror(err)
	fmt.Printf("ok\n")

	// Find start block
	startTime, err := utils.DateToTime(*startDate, 0, 0, 0)
	utils.Perror(err)
	startBlockHeader, err := utils.GetFirstBlockHeaderAtOrAfterTime(client, startTime)
	utils.Perror(err)
	tb1 := time.Unix(int64(startBlockHeader.Time), 0).UTC()
	fmt.Println("first block:", startBlockHeader.Number, tb1)

	// Find end block
	endTime, err := utils.DateToTime(*endDate, 0, 0, 0)
	utils.Perror(err)

	latestBlockHeader, err := utils.GetFirstBlockHeaderAtOrAfterTime(client, endTime)
	utils.Perror(err)
	tb2 := time.Unix(int64(latestBlockHeader.Time), 0).UTC()
	fmt.Println("last block: ", latestBlockHeader.Number, tb2)

	// fmt.Println("blocks", startBlock, "...", endBlock)
	timeStartBlockProcessing := time.Now()
	FindUncles(startBlockHeader.Number, latestBlockHeader.Number)

	// All done. Stop timer and print
	timeNeededBlockProcessing := time.Since(timeStartBlockProcessing)

	fmt.Printf("All done in %.3fs\n", timeNeededBlockProcessing.Seconds())
}

func FindUncles(startBlockNumber *big.Int, endBlockNumber *big.Int) {
	if startBlockNumber == nil || endBlockNumber == nil {
		log.Fatal("error: FindUncles blockNumber is nil")
	}

	blockChan := make(chan *types.Block, 100)

	// Start block processor
	var analyzeLock sync.Mutex
	go func() {
		analyzeLock.Lock()
		defer analyzeLock.Unlock() // we unlock when done

		for block := range blockChan {
			fbcommon.PrintBlock(block)
			blocksTotal += 1

			if _, found := minerStats[block.Coinbase()]; !found {
				minerStats[block.Coinbase()] = &MinerStat{Coinbase: block.Coinbase()}
			}

			minerStats[block.Coinbase()].NumBlocks += 1

			// Download the uncles for processing later
			for _, uncleHeader := range block.Uncles() {
				fmt.Printf("Downloading uncle %s...\n", uncleHeader.Hash())
				uncleBlock, err := client.BlockByHash(context.Background(), uncleHeader.Hash())
				if err != nil {
					fmt.Println("- error:", err)
					continue
				}
				uncles[uncleHeader.Hash()] = uncleBlock
			}
		}
	}()

	// Start getting blocks
	fbcommon.GetBlocks(blockChan, client, startBlockNumber.Int64(), endBlockNumber.Int64(), 15)

	// Wait until all blocks have been processed
	close(blockChan)
	analyzeLock.Lock()

	// Process the uncles: collect the mainchain block and the next one, and add to minerStats
	i := 0
	for _, uncleBlock := range uncles {
		if uncleBlock == nil {
			fmt.Println("err - block is nil")
			continue
		}

		if _, found := minerStats[uncleBlock.Coinbase()]; !found {
			minerStats[uncleBlock.Coinbase()] = &MinerStat{Coinbase: uncleBlock.Coinbase()}
		}

		// For each uncle, add stats
		minerStats[uncleBlock.Coinbase()].NumBlocks += 1
		minerStats[uncleBlock.Coinbase()].NumUncled += 1

		// get sibling block (mainchain block at height of uncleBlock)
		mainChainBlock, err := client.BlockByNumber(context.Background(), uncleBlock.Number())
		utils.Perror(err)
		if _, found := minerStats[mainChainBlock.Coinbase()]; !found {
			minerStats[mainChainBlock.Coinbase()] = &MinerStat{Coinbase: mainChainBlock.Coinbase()}
		}
		minerStats[mainChainBlock.Coinbase()].NumUnclings += 1

		// get next block
		nextHeight := new(big.Int).Add(uncleBlock.Number(), common.Big1)
		mainChainBlockChild1, err := client.BlockByNumber(context.Background(), nextHeight)
		utils.Perror(err)

		if mainChainBlock.Coinbase() == mainChainBlockChild1.Coinbase() {
			fmt.Printf("uncling+1 at %d by miner: %s (block %d / %d)\n", mainChainBlock.NumberU64(), mainChainBlock.Coinbase(), i, len(uncles))
			minerStats[mainChainBlock.Coinbase()].NumUnclingsPlus1 += 1
		}

		i += 1
	}

	// Compute percentage of unclings+1 compared to mined blocks by each miner
	for _, minerStat := range minerStats {
		if minerStat.NumBlocks == 0 {
			fmt.Println("x minerstat with 0 blocks:", minerStat.Coinbase, minerStat.NumBlocks, minerStat.NumUncled, minerStat.NumUnclingsPlus1)
		}
		minerStat.NumUnclingsPlus1PercentOfBlocks = float64(minerStat.NumUnclingsPlus1) / float64(minerStat.NumBlocks)
	}

	// Sort
	keys := make([]common.Address, 0, len(minerStats))
	for key := range minerStats {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return minerStats[keys[i]].NumUnclingsPlus1PercentOfBlocks > minerStats[keys[j]].NumUnclingsPlus1PercentOfBlocks
	})

	// Print sorted results
	for _, key := range keys {
		stat := minerStats[key]
		fmt.Printf("%s \t numBlocks=%4d, numUnclings=%4d, numUnclings+1=%4d \t perc=%.6f\n", key, stat.NumBlocks, stat.NumUnclings, stat.NumUnclingsPlus1, stat.NumUnclingsPlus1PercentOfBlocks)
	}
}
