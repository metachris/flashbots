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

var client *ethclient.Client
var mevGethUri string

var AddressLookup *addresslookup.AddressLookupService
var minerUncles map[common.Address]uint64 = make(map[common.Address]uint64)             // number of uncles per miner
var minersWhoHadMainSibling map[common.Address]uint64 = make(map[common.Address]uint64) // number of blocks that have an uncles, per miner
var minerBlockTotal map[common.Address]uint64 = make(map[common.Address]uint64)         // number of uncles per miner
var unclesTotal int
var blocksTotal int

func main() {
	var err error
	log.SetOutput(os.Stdout)

	AddressLookup = addresslookup.NewAddressLookupService(nil)
	err = AddressLookup.AddAllAddresses()
	utils.Perror(err)

	mevGethUri = *flag.String("eth", os.Getenv("MEVGETH_NODE"), "mev-geth node URI")
	startDate := flag.String("start", "", "time offset")
	endDate := flag.String("end", "", "time offset")
	// endDate := flag.String("end", "", "block number or time offset or date (yyyy-mm-dd) (optional)")
	flag.Parse()

	if mevGethUri == "" {
		log.Fatal("Missing eth node uri")
	}

	if *startDate == "" {
		log.Fatal("Missing start")
	}

	if *endDate == "" {
		log.Fatal("Missing end")
	}

	fmt.Printf("Connecting to %s ... ", mevGethUri)
	client, err = ethclient.Dial(mevGethUri)
	utils.Perror(err)
	fmt.Printf("ok\n")

	// Find start block
	startTime, err := utils.DateToTime(*startDate, 0, 0, 0)
	utils.Perror(err)
	// startTimeOffsetSec, err := fbcommon.TimeStringToSec(*startDate)
	// utils.Perror(err)
	// now := time.Now()
	// startTime := now.Add(time.Duration(startTimeOffsetSec) * time.Second * -1)
	startBlockHeader, err := utils.GetFirstBlockHeaderAtOrAfterTime(client, startTime)
	utils.Perror(err)
	tb1 := time.Unix(int64(startBlockHeader.Time), 0).UTC()
	fmt.Println("first block:", startBlockHeader.Number, tb1)

	// Find end block
	endTime, err := utils.DateToTime(*endDate, 0, 0, 0)
	utils.Perror(err)

	// latestBlockHeader, err := client.HeaderByNumber(context.Background(), nil)
	latestBlockHeader, err := utils.GetFirstBlockHeaderAtOrAfterTime(client, endTime)
	utils.Perror(err)
	tb2 := time.Unix(int64(latestBlockHeader.Time), 0).UTC()
	fmt.Println("last block: ", latestBlockHeader.Number, tb2)

	// fmt.Println("blocks", startBlock, "...", endBlock)
	timeStartBlockProcessing := time.Now()
	FindUncles(startBlockHeader.Number, latestBlockHeader.Number)

	// All done. Stop timer and print
	timeNeededBlockProcessing := time.Since(timeStartBlockProcessing)
	// PrintResult()
	PrintResultUncler()

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
			blocksTotal += 1
			fbcommon.PrintBlock(block)
			minerBlockTotal[block.Coinbase()] += 1
			if len(block.Uncles()) > 0 {
				// get ancestor of the uncle, the block who replaced the uncle in the chain
				parentBlockNumber := new(big.Int).Sub(block.Number(), common.Big1)
				parentBlock, err := client.BlockByNumber(context.Background(), parentBlockNumber)
				if err != nil {
					fmt.Println("- error: ", err)
				} else {
					minersWhoHadMainSibling[parentBlock.Coinbase()] += 1
				}

				for _, uncleHeader := range block.Uncles() {
					unclesTotal += 1
					minerUncles[uncleHeader.Coinbase] += 1
				}
			}
		}
	}()

	fbcommon.GetBlocks(blockChan, client, startBlockNumber.Int64(), endBlockNumber.Int64(), 15)

	close(blockChan)
	analyzeLock.Lock() // wait until all blocks have been processed
}

func PrintResult() {
	// sort miners by number of uncles
	keys := make([]common.Address, 0, len(minerUncles))
	for key := range minerUncles {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return minerBlockTotal[keys[i]] > minerBlockTotal[keys[j]] })

	for _, key := range keys {
		minerStr := key.Hex()
		minerAddr, found := AddressLookup.GetAddressDetail(key.Hex())
		if found {
			minerStr += fmt.Sprintf(" %s", minerAddr.Name)
		}

		numUncles := minerUncles[key]
		totalBlocks := minerBlockTotal[key]
		p := float64(numUncles) / float64(totalBlocks) * 100
		fmt.Printf("%-60s %3d / %5d = %6.2f %%\n", minerStr, numUncles, totalBlocks, p)
	}

	unclePercentage := float64(unclesTotal) / float64(blocksTotal) * 100
	fmt.Printf("%d uncles / %d valid blocks = %.2f %%\n", unclesTotal, blocksTotal, unclePercentage)
}

func PrintResultUncler() {
	fmt.Println("Summery of unclers")
	// sort miners by number of unclings
	keys := make([]common.Address, 0, len(minersWhoHadMainSibling))
	for key := range minersWhoHadMainSibling {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return minerBlockTotal[keys[i]] > minerBlockTotal[keys[j]] })

	for _, key := range keys {
		minerStr := key.Hex()
		minerAddr, found := AddressLookup.GetAddressDetail(key.Hex())
		if found {
			minerStr += fmt.Sprintf(" %s", minerAddr.Name)
		}

		numTotalBlocks := minerBlockTotal[key]
		numUnclings := minersWhoHadMainSibling[key]
		numUncles := minerUncles[key]

		unclingsPercent := float64(numUnclings) / float64(numTotalBlocks) * 100
		unclePercent := float64(numUncles) / float64(numTotalBlocks) * 100
		fmt.Printf("%-60s includedBlocks: %5d \t blocksWithSiblings: %4d (%6.2f %%) \t unclesProduced: %5d (%6.2f %%)\n", minerStr, numTotalBlocks, numUnclings, unclingsPercent, numUncles, unclePercent)
	}

	unclePercentage := float64(unclesTotal) / float64(blocksTotal) * 100
	fmt.Printf("%d uncles / %d valid blocks = %.2f %%\n", unclesTotal, blocksTotal, unclePercentage)
}
