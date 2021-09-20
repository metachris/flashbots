// Get miner rewards for a full block (via number or hash) by simulating it with eth_callBundle at an mev-geth instance.
// Note: The result already excludes the 2 ETH block reward and any burnt gas fees, it's the actual miner earnings from the transactions.
//
// Example arguments:
//
//     $ go run cmd/blocksim/main.go -mevgeth http://xxx.xxx.xxx.xxx:8545 -number 13100622
//     $ go run cmd/blocksim/main.go -mevgeth http://xxx.xxx.xxx.xxx:8545 -hash 0x662f81506bd1d1f7cbefa308261ba94ee63438998cdf085c95081448aaf4cc81
//
// Example output:
//
//     Connected to http://xxx.xxx.xxx.xxx:8545
//     Block 13100622 0x662f81506bd1d1f7cbefa308261ba94ee63438998cdf085c95081448aaf4cc81        2021-08-26 11:14:55 +0000 UTC   tx=99           gas=13854382    uncles=0
//     Simulation result:
//     - CoinbaseDiff:           67391709273784431     0.0674 ETH
//     - GasFees:                67391709273784431     0.0674 ETH
//     - EthSentToCoinbase:                      0     0.0000 ETH
//
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	flashbotsrpc "github.com/metachris/flashbots-rpc"
	fbcommon "github.com/metachris/flashbots/common"
	"github.com/metachris/go-ethutils/addresslookup"
	"github.com/metachris/go-ethutils/utils"
)

var addressLookup *addresslookup.AddressLookupService

func main() {
	mevGethUriPtr := flag.String("mevgeth", "", "mev-geth node URI")
	gethUriPtr := flag.String("geth", "", "geth node URI for tx lookup")
	blockHash := flag.String("hash", "", "hash of block to simulate")
	blockNumber := flag.Int64("number", -1, "number of block to simulate")
	debugPtr := flag.Bool("debug", false, "print debug information")
	flag.Parse()

	if *mevGethUriPtr == "" {
		log.Fatal("No mev geth URI provided")
	}

	if *blockHash == "" && *blockNumber == -1 {
		log.Fatal("Either block number or hash is needed")
	}

	mevGethClient, err := ethclient.Dial(*mevGethUriPtr)
	utils.Perror(err)
	fmt.Println("Connected to", *mevGethUriPtr)
	gethClient := mevGethClient

	if *gethUriPtr != "" && *gethUriPtr != *mevGethUriPtr {
		gethClient, err = ethclient.Dial(*gethUriPtr)
		utils.Perror(err)
		fmt.Println("Connected to", *gethUriPtr)
	}

	addressLookup = addresslookup.NewAddressLookupService(gethClient)
	err = addressLookup.AddAllAddresses()
	if err != nil {
		fmt.Println("addresslookup error:", err)
	}

	fmt.Println("Downloading block...")
	var block *types.Block
	var canonicalBlock *types.Block
	isCanonicalBlock := true

	if *blockHash != "" {
		hash := common.HexToHash(*blockHash)
		block, err = mevGethClient.BlockByHash(context.Background(), hash)
		utils.Perror(err)

		// Get block by number, to check if the block is from canonical chain or was reorg'ed
		canonicalBlock, err = mevGethClient.BlockByNumber(context.Background(), block.Number())
		utils.Perror(err)
		if block.Hash() != canonicalBlock.Hash() {
			isCanonicalBlock = false
		}

	} else {
		block, err = mevGethClient.BlockByNumber(context.Background(), big.NewInt(*blockNumber))
		utils.Perror(err)
	}

	fmt.Println("")
	printBlock(block)
	if !isCanonicalBlock {
		fmt.Print("- Block is not in canonical chain. Was replaced by:\n  ")
		printBlock(canonicalBlock)
	}

	if len(block.Transactions()) == 0 {
		fmt.Println("No transactions in this block")
		return
	}

	rpc := flashbotsrpc.NewFlashbotsRPC(*mevGethUriPtr)
	rpc.Debug = *debugPtr

	fmt.Println("\nSimulating block...")
	privateKey, _ := crypto.GenerateKey()
	result, err := rpc.FlashbotsSimulateBlock(privateKey, block, 0)
	utils.Perror(err)
	fmt.Println("Simulation result:")
	fmt.Printf("- CoinbaseDiff:      %22s %10s ETH\n", result.CoinbaseDiff, weiStrToEthStr(result.CoinbaseDiff, 4))
	fmt.Printf("- GasFees:           %22s %10s ETH\n", result.GasFees, weiStrToEthStr(result.GasFees, 4))
	fmt.Printf("- EthSentToCoinbase: %22s %10s ETH\n", result.EthSentToCoinbase, weiStrToEthStr(result.EthSentToCoinbase, 4))

	blockCbDiffWei := big.NewFloat(0)
	blockCbDiffWei, _ = blockCbDiffWei.SetString(result.CoinbaseDiff)

	// sort transactions by coinbasediff
	sort.Slice(result.Results, func(i, j int) bool {
		a := fbcommon.StrToBigInt(result.Results[i].CoinbaseDiff)
		b := fbcommon.StrToBigInt(result.Results[j].CoinbaseDiff)
		return a.Cmp(b) == 1
	})

	numTxNeededFor80PercentValue := 0
	currentValue := big.NewFloat(0)

	// If an address is receiving at least 2 tx, load the address info
	_addressUsed := make(map[string]bool)
	for _, entry := range result.Results {
		if _addressUsed[entry.ToAddress] {
			addressLookup.GetAddressDetail(entry.ToAddress)
		}
		_addressUsed[entry.ToAddress] = true
	}

	fmt.Println("\nTransactions:")
	for i, entry := range result.Results {
		_incl := ""
		if !isCanonicalBlock {
			// Where was this tx included?
			r, err := gethClient.TransactionReceipt(context.Background(), common.HexToHash(entry.TxHash))
			if err != nil {
				_incl = fmt.Sprintf("included-in: ? %s", err)
			} else {
				_incl = fmt.Sprintf("included-in: %s", r.BlockNumber)
			}
		}

		_to := entry.ToAddress
		if detail, found := addressLookup.Cache[strings.ToLower(entry.ToAddress)]; found {
			_to = fmt.Sprintf("%s (%s)", _to, detail.Name)
		}
		fmt.Printf("%4d %s cbD=%s, gasFee=%s, ethSentToCb=%s, to=%s \t %s\n", i+1, entry.TxHash, weiStrToEthStr(entry.CoinbaseDiff, 4), weiStrToEthStr(entry.GasFees, 4), weiStrToEthStr(entry.EthSentToCoinbase, 4), _to, _incl)

		cbDiffWei := new(big.Float)
		cbDiffWei, _ = cbDiffWei.SetString(entry.CoinbaseDiff)

		currentValue = new(big.Float).Add(currentValue, cbDiffWei)

		percentValueReached := new(big.Float).Quo(currentValue, blockCbDiffWei)
		// fmt.Println(percentValueReached.Text('f', 4))
		if numTxNeededFor80PercentValue == 0 && percentValueReached.Cmp(big.NewFloat(0.8)) > -1 {
			numTxNeededFor80PercentValue = i
		}
	}

	fmt.Printf("\n%d/%d tx needed for 80%% of miner value\n", numTxNeededFor80PercentValue, len(result.Results))
}

func printBlock(block *types.Block) {
	t := time.Unix(int64(block.Header().Time), 0).UTC()
	miner := block.Coinbase().Hex()
	if details, found := addressLookup.GetAddressDetail(block.Coinbase().Hex()); found {
		miner += " (" + details.Name + ")"
	}
	fmt.Printf("Block %d %s \t %s \t tx=%d, uncles=%d, miner: %s\n", block.NumberU64(), block.Hash(), t, len(block.Transactions()), len(block.Uncles()), miner)
}

func weiStrToEthStr(weiStr string, decimals int) string {
	i := new(big.Int)
	i.SetString(weiStr, 10)
	return utils.WeiBigIntToEthString(i, decimals)
}
