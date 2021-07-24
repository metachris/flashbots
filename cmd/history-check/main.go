package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/metachris/flashbots/blockcheck"
	"github.com/metachris/go-ethutils/blockswithtx"
	"github.com/metachris/go-ethutils/utils"
)

var errorSummary blockcheck.ErrorSummary = blockcheck.NewErrorSummary()

func main() {
	log.SetOutput(os.Stdout)

	ethUri := flag.String("eth", os.Getenv("ETH_NODE"), "Ethereum node URI")
	startDate := flag.String("start", "", "date (yyyy-mm-dd)")
	endDate := flag.String("end", "", "date (yyyy-mm-dd)")
	flag.Parse()

	if *startDate == "" || *endDate == "" {
		log.Fatal("Missing date")
	}

	if *ethUri == "" {
		log.Fatal("Missing eth node uri")
	}

	fmt.Printf("Connecting to %s ... ", *ethUri)
	client, err := ethclient.Dial(*ethUri)
	utils.Perror(err)
	fmt.Printf("ok\n")

	startTime, err := utils.DateToTime(*startDate, 0, 0, 0)
	utils.Perror(err)
	startBlockHeader, err := utils.GetFirstBlockHeaderAtOrAfterTime(client, startTime)
	utils.Perror(err)
	startBlock := startBlockHeader.Number.Int64()

	endTime, err := utils.DateToTime(*endDate, 0, 0, 0)
	// endTime, err := utils.DateToTime(*startDate, 5, 10, 0)
	utils.Perror(err)
	endBlockHeader, err := utils.GetFirstBlockHeaderAtOrAfterTime(client, endTime)
	utils.Perror(err)
	endBlock := endBlockHeader.Number.Int64()

	fmt.Println("blocks", startBlock, "...", endBlock)

	timestampMainStart := time.Now() // for measuring execution time

	// Prefetch Flashbots blocks
	fmt.Print("Caching flashbots blocks... ")
	blockcheck.CacheFlashbotsBlocks(startBlock, endBlock)
	fmt.Print("done\n")

	// Start fetching blocks
	blockChan := make(chan *blockswithtx.BlockWithTxReceipts, 100) // channel for resulting BlockWithTxReceipt

	// Start block processor
	var numBlocksProcessed int
	var numTxProcessed int
	var analyzeLock sync.Mutex
	go func() {
		analyzeLock.Lock()
		defer analyzeLock.Unlock() // we unlock when done

		for block := range blockChan {
			numBlocksProcessed += 1
			numTxProcessed += len(block.Block.Transactions())
			processBlockWithReceipts(block, client)
		}
	}()

	// Start fetching and processing blocks
	blockswithtx.GetBlocksWithTxReceipts(client, blockChan, startBlock, endBlock, 15)

	// Wait for processing to finish
	fmt.Println("Waiting for Analysis workers...")
	close(blockChan)
	analyzeLock.Lock() // wait until all blocks have been processed

	fmt.Println(errorSummary.String())

	timeNeeded := time.Since(timestampMainStart)
	fmt.Printf("Analysis of %s blocks, %s transactions finished in %.2fs\n", utils.NumberToHumanReadableString(numBlocksProcessed, 0), utils.NumberToHumanReadableString(numTxProcessed, 0), timeNeeded.Seconds())
}

func processBlockWithReceipts(block *blockswithtx.BlockWithTxReceipts, client *ethclient.Client) {
	utils.PrintBlock(block.Block)
	check, err := blockcheck.CheckBlock(block, true)
	utils.Perror(err)

	if check.HasSeriousErrors() || check.HasLessSeriousErrors() { // update and print miner error count on serious and less-serious errors
		errorSummary.AddCheckErrors(check)
	}
}
