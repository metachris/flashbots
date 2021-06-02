package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/metachris/go-ethutils/blockswithtx"
	"github.com/metachris/go-ethutils/utils"
)

var silent bool = false

func main() {
	ethUri := flag.String("eth", os.Getenv("ETH_NODE"), "Ethereum node URI")
	blockHeightPtr := flag.Int("block", 0, "specific block to check")
	datePtr := flag.String("date", "", "date (yyyy-mm-dd or -1d)")
	hourPtr := flag.Int("hour", 0, "hour (UTC)")
	minPtr := flag.Int("min", 0, "hour (UTC)")
	lenPtr := flag.String("len", "", "num blocks or timespan (4s, 5m, 1h, ...)")
	watchPtr := flag.Bool("watch", false, "watch and process new blocks")
	silentPtr := flag.Bool("silent", false, "don't print info about every block")
	flag.Parse()

	if *ethUri == "" {
		log.Fatal("Pass a valid eth node with -eth argument or ETH_NODE env var.")
	} else if !strings.HasPrefix(*ethUri, "/") {
		fmt.Printf("Warning: You should use a direct IPC connection to the Ethereum node, else it might be slow to download receipts for all transactions.\n")
	}

	client, err := ethclient.Dial(*ethUri)
	utils.Perror(err)
	silent = *silentPtr

	if *datePtr != "" || *blockHeightPtr != 0 {
		// A start for historical analysis was given
		// log.Fatal("Missing start (date or block). Add with -date <yyyy-mm-dd> or -block <blockNum>")
		startBlock, endBlock := getBlockRangeFromArguments(client, *blockHeightPtr, *datePtr, *hourPtr, *minPtr, *lenPtr)
		// fmt.Println(startBlock, endBlock)
		checkBlockRange(client, startBlock, endBlock)
	}

	if *watchPtr {
		watch(client)
	}
}

func getBlockRangeFromArguments(client *ethclient.Client, blockHeight int, date string, hour int, min int, length string) (startBlock int64, endBlock int64) {
	if date != "" && blockHeight != 0 {
		panic("cannot use both -block and -date arguments")
	}

	if date == "" && blockHeight == 0 {
		panic("need to use either -block or -date arguments")
	}

	// Parse -len argument. Can be either a number of blocks or a time duration (eg. 1s, 5m, 2h or 4d)
	numBlocks := 0
	timespanSec := 0
	var err error
	switch {
	case strings.HasSuffix(length, "s"):
		timespanSec, err = strconv.Atoi(strings.TrimSuffix(length, "s"))
		utils.Perror(err)
	case strings.HasSuffix(length, "m"):
		timespanSec, err = strconv.Atoi(strings.TrimSuffix(length, "m"))
		utils.Perror(err)
		timespanSec *= 60
	case strings.HasSuffix(length, "h"):
		timespanSec, err = strconv.Atoi(strings.TrimSuffix(length, "h"))
		utils.Perror(err)
		timespanSec *= 60 * 60
	case strings.HasSuffix(length, "d"):
		timespanSec, err = strconv.Atoi(strings.TrimSuffix(length, "d"))
		utils.Perror(err)
		timespanSec *= 60 * 60 * 24
	case length == "": // default 1 block
		numBlocks = 1
	default: // No suffix: number of blocks
		numBlocks, err = strconv.Atoi(length)
		utils.Perror(err)
	}

	// startTime is set from date argument, or block timestamp if -block argument was used
	var startTime time.Time

	// Get start block
	if blockHeight > 0 { // start at block height
		startBlockHeader, err := client.HeaderByNumber(context.Background(), big.NewInt(int64(blockHeight)))
		utils.Perror(err)
		startBlock = startBlockHeader.Number.Int64()
		startTime = time.Unix(int64(startBlockHeader.Time), 0)
	} else {
		// Negative date prefix (-1d, -2m, -1y)
		if strings.HasPrefix(date, "-") {
			if strings.HasSuffix(date, "d") {
				t := time.Now().AddDate(0, 0, -1)
				startTime = t.Truncate(24 * time.Hour)
			} else if strings.HasSuffix(date, "m") {
				t := time.Now().AddDate(0, -1, 0)
				startTime = t.Truncate(24 * time.Hour)
			} else if strings.HasSuffix(date, "y") {
				t := time.Now().AddDate(-1, 0, 0)
				startTime = t.Truncate(24 * time.Hour)
			} else {
				panic(fmt.Sprintf("Not a valid date offset: '%s'. Can be d, m, y", date))
			}
		} else {
			startTime, err = utils.DateToTime(date, hour, min, 0)
			utils.Perror(err)
		}

		startBlockHeader, err := utils.GetFirstBlockHeaderAtOrAfterTime(client, startTime)
		utils.Perror(err)
		startBlock = startBlockHeader.Number.Int64()
	}

	if numBlocks > 0 {
		endBlock = startBlock + int64(numBlocks-1)
	} else if timespanSec > 0 {
		endTime := startTime.Add(time.Duration(timespanSec) * time.Second)
		// fmt.Printf("endTime: %v\n", endTime.UTC())
		endBlockHeader, _ := utils.GetFirstBlockHeaderAtOrAfterTime(client, endTime)
		endBlock = endBlockHeader.Number.Int64() - 1
	} else {
		panic("No valid block range")
	}

	if endBlock < startBlock {
		endBlock = startBlock
	}

	return startBlock, endBlock
}

func checkBlockRange(client *ethclient.Client, startHeight int64, endHeight int64) {
	fmt.Printf("Checking blocks %d to %d...\n", startHeight, endHeight)
	t1 := time.Now()
	blockChan := make(chan *blockswithtx.BlockWithTxReceipts, 100) // channel for resulting BlockWithTxReceipt

	// Start thread listening for blocks (with tx receipts) from geth worker pool
	var numTx int64
	var lock sync.Mutex
	go func() {
		lock.Lock()
		defer lock.Unlock()
		for b := range blockChan {
			checkBlockWithReceipts(b)
			numTx += int64(len(b.Block.Transactions()))
		}
	}()

	// Start fetching and processing blocks
	blockswithtx.GetBlocksWithTxReceipts(client, blockChan, startHeight, endHeight, 5)

	// Wait for processing to finish
	close(blockChan)
	lock.Lock() // wait until all blocks have been processed

	// All done
	t2 := time.Since(t1)
	fmt.Printf("Processed %s blocks (%s transactions) in %.3f seconds\n", utils.NumberToHumanReadableString(endHeight-startHeight+1, 0), utils.NumberToHumanReadableString(numTx, 0), t2.Seconds())
}

func watch(client *ethclient.Client) {
	headers := make(chan *types.Header)
	sub, err := client.SubscribeNewHead(context.Background(), headers)
	if err != nil {
		log.Fatal(err)
	}

	for {
		select {
		case err := <-sub.Err():
			log.Fatal(err)
		case header := <-headers:
			b, err := blockswithtx.GetBlockWithTxReceipts(client, header.Number.Int64())
			utils.Perror(err)
			checkBlockWithReceipts(b)
		}
	}
}

func checkBlockWithReceipts(b *blockswithtx.BlockWithTxReceipts) {
	if !silent {
		utils.PrintBlock(b.Block)
	}

	for _, tx := range b.Block.Transactions() {
		receipt := b.TxReceipts[tx.Hash()]
		if receipt == nil {
			continue
		}

		if utils.IsBigIntZero(tx.GasPrice()) && len(tx.Data()) > 0 {
			sender, _ := utils.GetTxSender(tx)
			if receipt.Status == 1 { // successful tx
				// fmt.Printf("Flashbots tx in block %v: %s from %v\n", b.Block.Number(), tx.Hash(), sender)
			} else { // failed tx
				// fmt.Printf("block %v: failed Flashbots tx %s from %v\n", WarningColor, b.Block.Number(), tx.Hash(), sender)
				utils.ColorPrintf(utils.ErrorColor, "block %v: failed Flashbots tx %s from %v\n", b.Block.Number(), tx.Hash(), sender)
			}
		}
	}
}
