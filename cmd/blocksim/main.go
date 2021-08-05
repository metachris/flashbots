package main

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/metachris/flashbots/api/ethrpc"
	"github.com/metachris/go-ethutils/utils"
)

// Old path (before reorg):
// - 12952442 0xbb3d9344bd0107b5c5f29aefcbf9c79bf1781030d8bfe5399fa51dbc6b4124fb (now uncle)
// - 12952443 0x6285374dacc9cc073076d946326fb28448ddde6ba5aaf4da76def1e5b2552833 (child path of uncle)

func main() {
	ethUri := os.Getenv("ETH_NODE")
	client, err := ethclient.Dial(ethUri)
	utils.Perror(err)
	fmt.Println("Connected to", ethUri)

	hash := common.HexToHash("0x85067ef00dfa9bacbdff605ae90fe5f586a0d4df4d8d1a11ed82dcef771c2450")
	block, err := client.BlockByHash(context.Background(), hash)
	utils.Perror(err)
	SimulateBlock(block)
}

// func SimulateBlock(client *ethclient.Client, blockHash common.Hash) {
func SimulateBlock(block *types.Block) {
	fmt.Printf("Simulating block %s 0x%x %s \t %d tx \t timestamp: %d\n", block.Number(), block.Number(), block.Header().Hash(), len(block.Transactions()), block.Header().Time)

	txs := make([]string, 0)
	for _, tx := range block.Transactions() {
		if tx.Type() != 0 {
			continue
		}

		rlp := TxToRlp(tx)
		txs = append(txs, rlp)

		// if len(txs) == 50 {
		// 	break
		// }
	}

	param := ethrpc.FlashbotsCallBundleParam{
		Txs:              txs,
		BlockNumber:      fmt.Sprintf("0x%x", block.Number()),
		StateBlockNumber: block.ParentHash().Hex(),
	}

	ethUri := "https://relay.flashbots.net"
	rpcClient := ethrpc.New(ethUri)
	rpcClient.Debug = true

	privateKey, _ := crypto.GenerateKey()
	result, err := rpcClient.FlashbotsCallBundle(privateKey, param)
	utils.Perror(err)
	fmt.Println(result)
}

func TxToRlp(tx *types.Transaction) string {
	var buff bytes.Buffer
	tx.EncodeRLP(&buff)
	return fmt.Sprintf("0x%x", buff.Bytes())
}
