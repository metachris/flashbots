package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	flashbotsrpc "github.com/metachris/flashbots-rpc"
	"github.com/metachris/go-ethutils/utils"
)

func main() {
	mevGethUriPtr := flag.String("mevgeth", os.Getenv("MEVGETH_NODE"), "mev-geth node URI")
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

	client, err := ethclient.Dial(*mevGethUriPtr)
	utils.Perror(err)
	fmt.Println("Connected to", *mevGethUriPtr)

	var block *types.Block
	if *blockHash != "" {
		hash := common.HexToHash(*blockHash)
		block, err = client.BlockByHash(context.Background(), hash)
		utils.Perror(err)

	} else {
		block, err = client.BlockByNumber(context.Background(), big.NewInt(*blockNumber))
		utils.Perror(err)
	}

	t := time.Unix(int64(block.Header().Time), 0).UTC()
	fmt.Printf("Block %d %s \t %s \t tx=%-4d \t gas=%d \t uncles=%d\n", block.NumberU64(), block.Hash(), t, len(block.Transactions()), block.GasUsed(), len(block.Uncles()))

	if len(block.Transactions()) == 0 {
		fmt.Println("No transactions in this block")
		return
	}

	rpc := flashbotsrpc.NewFlashbotsRPC(*mevGethUriPtr)
	rpc.Debug = *debugPtr

	privateKey, _ := crypto.GenerateKey()
	result, err := rpc.FlashbotsSimulateBlock(privateKey, block, 0)
	utils.Perror(err)
	fmt.Println("Simulation result:")
	fmt.Printf("- CoinbaseDiff:      %22s %10s ETH\n", result.CoinbaseDiff, weiStrToEthStr(result.CoinbaseDiff, 4))
	fmt.Printf("- GasFees:           %22s %10s ETH\n", result.GasFees, weiStrToEthStr(result.GasFees, 4))
	fmt.Printf("- EthSentToCoinbase: %22s %10s ETH\n", result.EthSentToCoinbase, weiStrToEthStr(result.EthSentToCoinbase, 4))
}

func weiStrToEthStr(weiStr string, decimals int) string {
	i := new(big.Int)
	i.SetString(weiStr, 10)
	return utils.WeiBigIntToEthString(i, decimals)
}
