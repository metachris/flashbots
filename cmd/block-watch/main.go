// Watch blocks and report issues (to terminal and to Discord)
//
// Issues:
// 1. Failed Flashbots (or other 0-gas) transaction
// 2. Bundle out of order by effective-gasprice
// 3. Bundle effective-gasprice is lower than lowest non-fb tx gasprice
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/metachris/flashbots/blockcheck"
	"github.com/metachris/go-ethutils/blockswithtx"
	"github.com/metachris/go-ethutils/utils"
)

const BundlePercentPriceDiffThreshold float32 = 50

var silent bool
var sendErrorsToDiscord bool

var BlockBacklog map[int64]*blockswithtx.BlockWithTxReceipts = make(map[int64]*blockswithtx.BlockWithTxReceipts)

func main() {
	log.SetOutput(os.Stdout)

	ethUri := flag.String("eth", os.Getenv("ETH_NODE"), "Ethereum node URI")
	// recentBundleOrdersPtr := flag.Bool("recentBundleOrder", false, "check recent bundle orders blocks")
	blockHeightPtr := flag.Int64("block", 0, "specific block to check")
	watchPtr := flag.Bool("watch", false, "watch and process new blocks")
	silentPtr := flag.Bool("silent", false, "don't print info about every block")
	discordPtr := flag.Bool("discord", false, "send errors to Discord")
	// webserverPtr := flag.String("webserver", ":6069", "run webserver on this port")
	flag.Parse()

	silent = *silentPtr

	if *discordPtr {
		if len(os.Getenv("DISCORD_WEBHOOK")) == 0 {
			log.Fatal("No DISCORD_WEBHOOK environment variable found!")
		}
		sendErrorsToDiscord = *discordPtr
	}

	// Connect to the geth node and start the BlockCheckService
	if *ethUri == "" {
		log.Fatal("Pass a valid eth node with -eth argument or ETH_NODE env var.")
	}

	client, err := ethclient.Dial(*ethUri)
	utils.Perror(err)

	if *blockHeightPtr != 0 {
		block, err := blockswithtx.GetBlockWithTxReceipts(client, *blockHeightPtr)
		utils.Perror(err)

		check := blockcheck.CheckBlock(block)
		msg := check.Sprint(true, false)
		fmt.Println(msg)
	}

	if *watchPtr {
		watch(*ethUri)
	}
}

func watch(ethUri string) {
	// client, err := ethclient.Dial(ethUri)
	// utils.Perror(err)

	// headers := make(chan *types.Header)
	// sub, err := client.SubscribeNewHead(context.Background(), headers)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// // Start the webserver
	// // go func() {
	// // 	http.HandleFunc("/failedTx", failedTxHistoryHandler)
	// // 	log.Fatal(http.ListenAndServe(webserverAddr, nil))
	// // }()

	// for {
	// 	select {
	// 	case err := <-sub.Err():
	// 		log.Fatal(err)
	// 	case header := <-headers:
	// 		b, err := blockswithtx.GetBlockWithTxReceipts(client, header.Number.Int64())
	// 		utils.Perror(err)

	// 		if !silent {
	// 			fmt.Println("Queueing new block", b.Block.Number())
	// 		}

	// 		// Add to backlog
	// 		BlockBacklog[header.Number.Int64()] = b

	// 		// Query flashbots API to get latest block it has processed
	// 		opts := api.GetBlocksOptions{BlockNumber: header.Number.Int64()}
	// 		flashbotsResponse, err := api.GetBlocks(&opts)
	// 		if err != nil {
	// 			log.Println("error:", err)
	// 			continue
	// 		}

	// 		// Process all blocks from the backlog which are already processed by the Flashbots API
	// 		for height, blockFromBacklog := range BlockBacklog {
	// 			if height <= flashbotsResponse.LatestBlockNumber {
	// 				if !silent {
	// 					utils.PrintBlock(blockFromBacklog.Block)
	// 				}

	// 				CheckBlockForFailedTx(blockFromBacklog)
	// 				checkBundleOrderDone := CheckBundles(blockFromBacklog)

	// 				// Success, remove from backlog
	// 				if checkBundleOrderDone {
	// 					delete(BlockBacklog, blockFromBacklog.Block.Number().Int64())
	// 				}
	// 			}
	// 		}
	// 	}
	// }
}

// // BUNDLE CHECKS
// func CheckBundles(block *blockswithtx.BlockWithTxReceipts) (checkCompleted bool) {
// 	// Check for bundle-out-of-order errors
// 	fbBlock, checkCompleted := CheckBlockForBundleOrderErrors(block.Block.Number().Int64())
// 	if !checkCompleted {
// 		return false
// 	}

// 	// If there are no flashbots bundles in this block, fbBlock will be nil
// 	if fbBlock == nil {
// 		return true
// 	}

// 	// Check bundle effective gas price > lowest tx gas price
// 	// 1. find lowest non-fb-tx gas price
// 	// 2. compare all fb-tx effective gas prices
// 	lowestGasPrice := big.NewInt(-1)
// 	lowestGasPriceTxHash := ""
// 	for _, tx := range block.Block.Transactions() {
// 		isFlashbotsTx, _, err := flashbotsutils.IsFlashbotsTx(block.Block, tx)
// 		utils.Perror(err)

// 		if isFlashbotsTx {
// 			continue
// 		}

// 		if lowestGasPrice.Int64() == -1 || tx.GasPrice().Cmp(lowestGasPrice) == -1 {
// 			lowestGasPrice = tx.GasPrice()
// 			lowestGasPriceTxHash = tx.Hash().Hex()
// 		}
// 	}

// 	for _, b := range fbBlock.Bundles {
// 		if b.RewardDivGasUsed.Cmp(lowestGasPrice) == -1 {
// 			// calculate percent difference:
// 			fCur := new(big.Float).SetInt(b.RewardDivGasUsed)
// 			fLow := new(big.Float).SetInt(lowestGasPrice)
// 			diffPercent1 := new(big.Float).Quo(fCur, fLow)
// 			diffPercent2 := new(big.Float).Sub(big.NewFloat(1), diffPercent1)
// 			diffPercent := new(big.Float).Mul(diffPercent2, big.NewFloat(100))

// 			fmt.Printf("Bundle %d in block %d has %s%% lower effective-gas-price (%v) than lowest non-fb transaction (%v)\n", b.Index, fbBlock.Number, diffPercent.Text('f', 2), common.BigIntToEString(b.RewardDivGasUsed, 4), common.BigIntToEString(lowestGasPrice, 4))
// 			if diffPercent.Cmp(big.NewFloat(49)) == 1 {
// 				if sendErrorsToDiscord {
// 					msg := fmt.Sprintf("Bundle %d in block [%d](<https://etherscan.io/block/%d>) ([bundle-explorer](<https://flashbots-explorer.marto.lol/?block=%d>)) has %s%% lower effective_gas_price (%v) than lowest non-fb [transaction](<https://etherscan.io/tx/%s>) (%v). Miner: [%s](<https://etherscan.io/address/%s>)\n", b.Index, fbBlock.Number, fbBlock.Number, fbBlock.Number, diffPercent.Text('f', 2), common.BigIntToEString(b.RewardDivGasUsed, 4), lowestGasPriceTxHash, common.BigIntToEString(lowestGasPrice, 4), fbBlock.Miner, fbBlock.Miner)
// 					SendToDiscord(msg)
// 				}
// 			}
// 		}
// 	}

// 	return true
// }
