// Watch blocks and check for errors
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/metachris/flashbots/api"
	"github.com/metachris/flashbots/bundleorder"
	"github.com/metachris/flashbots/failedtx"
	"github.com/metachris/flashbots/flashbotsutils"
	"github.com/metachris/go-ethutils/blockswithtx"
	"github.com/metachris/go-ethutils/utils"
)

var silent bool
var sendErrorsToDiscord bool

func main() {
	log.SetOutput(os.Stdout)

	ethUri := flag.String("eth", os.Getenv("ETH_NODE"), "Ethereum node URI")
	recentBundleOrdersPtr := flag.Bool("recentBundleOrder", false, "check recent bundle orders blocks")
	blockHeightPtr := flag.Int64("block", 0, "specific block to check")
	watchPtr := flag.Bool("watch", false, "watch and process new blocks")
	silentPtr := flag.Bool("silent", false, "don't print info about every block")
	discordPtr := flag.Bool("discord", false, "send errors to Discord")
	webserverPtr := flag.String("webserver", ":6067", "don't print info about every block")
	flag.Parse()

	silent = *silentPtr
	sendErrorsToDiscord = *discordPtr

	if *recentBundleOrdersPtr {
		CheckRecentBundles()
	}

	if *blockHeightPtr > 0 {
		CheckBlockForBundleOrderErrors(*blockHeightPtr)
	}

	if *watchPtr {
		watch(*ethUri, *webserverPtr)
	}
}

// BlockBacklog is used in watch mode: new blocks are added to the backlog until they are processed by the Flashbots backend (the API has ~5 blocks delay)
var BlockBacklog map[int64]*blockswithtx.BlockWithTxReceipts = make(map[int64]*blockswithtx.BlockWithTxReceipts)

// FailedTxHistory is used to serve the most recent failed tx via the webserver
var FailedTxHistory []failedtx.BlockWithFailedTx = make([]failedtx.BlockWithFailedTx, 0, 100)

func watch(ethUri, webserverAddr string) {
	if ethUri == "" {
		log.Fatal("Pass a valid eth node with -eth argument or ETH_NODE env var.")
	} else if !strings.HasPrefix(ethUri, "/") {
		fmt.Printf("Warning: You should use a direct IPC connection to the Ethereum node, else it might be slow to download receipts for all transactions.\n")
	}

	client, err := ethclient.Dial(ethUri)
	utils.Perror(err)

	headers := make(chan *types.Header)
	sub, err := client.SubscribeNewHead(context.Background(), headers)
	if err != nil {
		log.Fatal(err)
	}

	// Start the webserver
	go func() {
		http.HandleFunc("/failedTx", failedTxHistoryHandler)
		log.Fatal(http.ListenAndServe(webserverAddr, nil))
	}()

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
			BlockBacklog[header.Number.Int64()] = b

			// Query flashbots API to get latest block it has processed
			opts := api.GetBlocksOptions{BlockNumber: header.Number.Int64()}
			flashbotsResponse, err := api.GetBlocks(&opts)
			if err != nil {
				log.Println("error:", err)
				continue
			}

			// Process all blocks from the backlog which are already processed by the Flashbots API
			for height, blockFromBacklog := range BlockBacklog {
				if height <= flashbotsResponse.LatestBlockNumber {
					if !silent {
						utils.PrintBlock(blockFromBacklog.Block)
					}

					// checkBlockForFailedTx(blockFromBacklog)
					checkComplete := CheckBlockForBundleOrderErrors(blockFromBacklog.Block.Number().Int64())

					// Success, remove from backlog
					if checkComplete {
						delete(BlockBacklog, blockFromBacklog.Block.Number().Int64())
					}
				}
			}
		}
	}
}

//
// CHECK BUNDLE ORDERING
//
func CheckBlockForBundleOrderErrors(blockNumber int64) (checkComplete bool) {
	flashbotsBlocks, err := api.GetBlocks(&api.GetBlocksOptions{BlockNumber: blockNumber})
	if err != nil {
		log.Println(err)
		return false
	}

	if len(flashbotsBlocks.Blocks) != 1 {
		if len(flashbotsBlocks.Blocks) == 0 { // no flashbots tx in this block
			return true
		}
		fmt.Printf("- error fetching flashbots block %d - expected 1 block, got %d\n", blockNumber, len(flashbotsBlocks.Blocks))
		return false
	}

	b := bundleorder.CheckBlock(flashbotsBlocks.Blocks[0])
	if b.HasErrors() {
		bundleorder.PrintBlock(b)
		fmt.Println("")

		// send to Discord
		if sendErrorsToDiscord {
			SendBlockErrorToDiscord(b)
		}
	}
	return true
}

func CheckRecentBundles() {
	blocks, err := api.GetBlocks(&api.GetBlocksOptions{Limit: 10_000})
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
		b := bundleorder.CheckBlock(block)
		if b.HasErrors() {
			bundleorder.PrintBlock(b)
			fmt.Println("")
		}
	}
}

//
// FAILED TX
//
func CheckBlockForFailedTx(block *blockswithtx.BlockWithTxReceipts) {
	failedTransactions := _checkBlockForFailedTx(block)
	if len(failedTransactions) > 0 {
		// Append to failed-tx history
		FailedTxHistory = append(FailedTxHistory, failedtx.BlockWithFailedTx{
			BlockHeight: block.Block.Number().Int64(),
			FailedTx:    failedTransactions,
		})
		if len(FailedTxHistory) == 100 { // truncate history
			FailedTxHistory = FailedTxHistory[1:]
		}
	}
}

// Cache for last Flashbots API call (avoids calling multiple times per block)
type FlashbotsApiReqRes struct {
	RequestBlock int64
	Response     api.GetBlocksResponse
}

func _checkBlockForFailedTx(b *blockswithtx.BlockWithTxReceipts) (failedTx []failedtx.FailedTx) {
	failedTx = make([]failedtx.FailedTx, 0)

	// FlashbotsApiResponseCache is used to avoid querying the Flashbots API multiple times for failed transactions within a single block
	var flashbotsApiResponseCache FlashbotsApiReqRes

	// Iterate over all transactions in this block
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
				// Check if is Flashbots tx
				isFlashbotsTx := false

				// Either the Flashbots API response is already cached, or we do the API call now
				if flashbotsApiResponseCache.RequestBlock == b.Block.Number().Int64() {
					isFlashbotsTx = flashbotsApiResponseCache.Response.HasTx(tx.Hash().String())

				} else {
					var response api.GetBlocksResponse
					var err error
					isFlashbotsTx, response, err = flashbotsutils.IsFlashbotsTx(b.Block, tx)
					if err != nil {
						log.Println("Error:", err)
						return failedTx
					}

					flashbotsApiResponseCache.RequestBlock = b.Block.Number().Int64()
					flashbotsApiResponseCache.Response = response
				}

				// Create a FailedTx instance for this transaction
				var to string
				if tx.To() != nil {
					to = tx.To().String()
				}
				failedTx = append(failedTx, failedtx.FailedTx{
					Hash:        tx.Hash().String(),
					From:        sender.String(),
					To:          to,
					Block:       b.Block.Number().Uint64(),
					IsFlashbots: isFlashbotsTx,
				})

				// Print to terminal
				if isFlashbotsTx {
					utils.ColorPrintf(utils.ErrorColor, "failed Flashbots tx %s from %v in block %s\n", tx.Hash(), sender, b.Block.Number())
				} else {
					utils.ColorPrintf(utils.WarningColor, "failed 0-gas tx %s from %v in block %s\n", tx.Hash(), sender, b.Block.Number())
				}
			}
		}
	}

	return failedTx
}

func failedTxHistoryHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(FailedTxHistory)
}
