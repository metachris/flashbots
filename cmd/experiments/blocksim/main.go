package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/metachris/flashbots/api/ethrpc"
	fbcommon "github.com/metachris/flashbots/common"
	"github.com/metachris/go-ethutils/utils"
)

var mevGethNode string = os.Getenv("MEVGETH_NODE")
var gethNode string = fbcommon.EnvStr("GETH_NODE", mevGethNode)

// Example reorg:
// - 12952442 0xbb3d9344bd0107b5c5f29aefcbf9c79bf1781030d8bfe5399fa51dbc6b4124fb (now uncle)
// - 12952443 0x6285374dacc9cc073076d946326fb28448ddde6ba5aaf4da76def1e5b2552833 (child path of uncle)

// Example blocks ok:
// 12973115 0x3b0ea817cdc017c05904f030f668e47694b4564e8c41efa020dc3474ada75e91	tx: 9
// 12973112 0xf9b1fc1989c22c2502dc29e48eb95a7abeedfab12c8bf02e072b982d88962262  tx: 184/171

// Example blocks error:
// 12973114 0x674370e10581bf43b88af5263970d73ab9feef39563d72252e95e8fa341b7023  tx: 159, error: nonce too high
// 12973113 0x2e5b87f68aeba0dfc1b4abb96fece3b396064a9b97eb03f739755ffb349f5149  tx: 177  error: #92, nonce too high

func main() {
	blockHash := flag.String("blockhash", "", "hash of block to simulate")
	flag.Parse()
	// fmt.Println(1, *bl)
	if *blockHash == "" {
		log.Fatal("No block hash given")
	}

	client, err := ethclient.Dial(gethNode)
	utils.Perror(err)
	fmt.Println("Connected to", gethNode)

	hash := common.HexToHash(*blockHash)
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

		rlp := fbcommon.TxToRlp(tx)
		txs = append(txs, rlp)

		// if len(txs) == 92 {
		// 	break
		// }
	}

	fmt.Printf("- %d tx sending for simulation to %s...\n", len(txs), mevGethNode)

	param := ethrpc.FlashbotsCallBundleParam{
		Txs:              txs,
		BlockNumber:      fmt.Sprintf("0x%x", block.Number()),
		StateBlockNumber: block.ParentHash().Hex(),
	}

	rpcClient := ethrpc.New(mevGethNode)
	// rpcClient.Debug = true

	privateKey, _ := crypto.GenerateKey()
	result, err := rpcClient.FlashbotsCallBundle(privateKey, param)
	utils.Perror(err)
	// fmt.Println(result)
	fmt.Println("Coinbase diff:", result.CoinbaseDiff)

}
