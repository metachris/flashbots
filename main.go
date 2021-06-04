package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
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
		startBlock, endBlock := getBlockRangeFromArguments(client, *blockHeightPtr, *datePtr, *hourPtr, *minPtr, *lenPtr)
		checkBlockRange(client, startBlock, endBlock)
	} else if *watchPtr {
		watch(client)
	} else {
		fmt.Println("Nothing to do, check the help with -h.")
	}
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
			checkBlock(b)
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

var blockBacklog map[int64]*blockswithtx.BlockWithTxReceipts = make(map[int64]*blockswithtx.BlockWithTxReceipts)

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

			if !silent {
				fmt.Println("Queueing new block", b.Block.Number())
			}

			// Add to backlog
			blockBacklog[header.Number.Int64()] = b

			// Query flashbots API to get latest block it has processed
			flashbotsResponse, err := GetFlashbotsBlock(header.Number.Int64())
			if err != nil {
				log.Println("error:", err)
				continue
			}
			// fmt.Println("- Flashbots API latest:", flashbotsResponse.LatestBlockNumber)

			// Process all possible blocks in the backlog
			for height, backlogBlock := range blockBacklog {
				if height <= flashbotsResponse.LatestBlockNumber {
					checkBlock(backlogBlock)
				}
			}
		}
	}
}

func checkBlock(b *blockswithtx.BlockWithTxReceipts) {
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
				isFlashbotsTx, err := IsFlashbotsTx(b.Block, tx)
				if err != nil {
					log.Println("Error:", err)
					return
				}

				if isFlashbotsTx {
					utils.ColorPrintf(utils.ErrorColor, "failed Flashbots tx %s from %v in block %s\n", tx.Hash(), sender, b.Block.Number())
				} else {
					utils.ColorPrintf(utils.WarningColor, "failed 0-gas tx %s from %v in block %s\n", tx.Hash(), sender, b.Block.Number())
				}
			}
		}
	}

	delete(blockBacklog, b.Block.Number().Int64())
}
