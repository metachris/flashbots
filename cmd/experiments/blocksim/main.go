package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"math/big"
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
var debug bool

func main() {
	var err error

	blockHash := flag.String("blockhash", "", "hash of block to simulate")
	blockNumber := flag.Int64("blocknumber", -1, "number of block to simulate")
	debugPtr := flag.Bool("debug", false, "print debug information")
	flag.Parse()

	debug = *debugPtr

	if *blockHash == "" && *blockNumber < 0 {
		log.Fatal("No block given")
	}

	client, err := ethclient.Dial(gethNode)
	utils.Perror(err)
	fmt.Println("Connected to", gethNode)

	var block *types.Block
	if *blockHash != "" {
		hash := common.HexToHash(*blockHash)
		block, err = client.BlockByHash(context.Background(), hash)
		utils.Perror(err)

	} else {
		if *blockNumber == 0 {
			block, err = client.BlockByNumber(context.Background(), nil)
			utils.Perror(err)
		} else if *blockNumber > 0 {
			block, err = client.BlockByNumber(context.Background(), big.NewInt(*blockNumber))
			utils.Perror(err)
		}

		if len(block.Transactions()) == 0 {
			fmt.Println("No transactions in this block")
			return
		}

		// utils.Perror(err)
		// breakingTxIndex := FindBreakingTx(block)
		// if breakingTxIndex >= 0 {
		// 	_tx := block.Transactions()[breakingTxIndex]
		// 	fmt.Println("\nblock", block.Number(), "has breaking tx at index", breakingTxIndex, _tx.Hash(), "- type:", _tx.Type())
		// 	fmt.Println("RLP:", fbcommon.TxToRlp(_tx))
		// 	for i, v := range block.Transactions()[:breakingTxIndex+1] {
		// 		fmt.Printf("%3d %s %d \n", i, v.Hash(), v.Type())
		// 	}
		// }
	}

	result, err := SimulateBlock(block, 0, debug)
	utils.Perror(err)
	earnings := new(big.Int)
	earnings.SetString(result.CoinbaseDiff, 10)

	// Iterate over all transactions - add sent value back into earnings, remove received value
	for _, tx := range block.Transactions() {
		from, fromErr := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
		to := tx.To()
		txIsFromCoinbase := fromErr == nil && from == block.Coinbase()
		txIsToCoinbase := to != nil && *to == block.Coinbase()

		// Check if sent from coinbase address to somewhere else
		if txIsFromCoinbase {
			fmt.Println("outgoing tx from", from, "to", to)
			earnings = new(big.Int).Add(earnings, tx.Value())
		}

		// Check if received at coinbase address from somewhere else
		if txIsToCoinbase {
			fmt.Println("incoming tx from", from, "to", to)
			earnings = new(big.Int).Sub(earnings, tx.Value())
		}
	}

	// totalEarningsViaMevGeth := new(big.Int).Add(earnings, twoEthInWei)

	fmt.Printf("callBundle sim: %d/%d tx, block %d %s\n", len(block.Transactions()), len(result.Results), block.NumberU64(), block.Hash())
	fmt.Printf("- result.CoinbaseDiff:      %22s wei %24s ETH\n", result.CoinbaseDiff, utils.WeiBigIntToEthString(earnings, 10))
	fmt.Printf("- result.GasFees:           %22s wei %24s ETH\n", result.GasFees, WeiStrToEth(result.GasFees))
	fmt.Printf("- result.EthSentToCoinbase: %22s wei %24s ETH\n", result.EthSentToCoinbase, WeiStrToEth(result.EthSentToCoinbase))
	// fmt.Printf("- totalEarnings: %28s / %s ETH\n", totalEarningsViaMevGeth, utils.WeiBigIntToEthString(totalEarningsViaMevGeth, 10))

	client2, err := ethclient.Dial(mevGethNode)
	utils.Perror(err)
	es := fbcommon.NewEarningsService(client2)
	earningsServiceResult, err := es.GetBlockCoinbaseEarnings(block)
	utils.Perror(err)

	twoEthInWei := new(big.Int).Mul(common.Big2, big.NewInt(int64(math.Pow10(18))))
	minerFeesTotal := new(big.Int).Sub(earningsServiceResult, twoEthInWei)

	fmt.Printf("\nearningsService: %33s wei %24s ETH\n", minerFeesTotal.String(), utils.WeiBigIntToEthString(minerFeesTotal, 10))

	if minerFeesTotal.Cmp(earnings) == 0 {
		utils.ColorPrintf(fbcommon.ColorGreen, "Results match!\n")
	} else {
		utils.ColorPrintf(utils.ErrorColor, "mismatch!\n")
	}
}

func WeiStrToEth(w string) string {
	bi := new(big.Int)
	bi.SetString(w, 10)
	return utils.WeiBigIntToEthString(bi, 10)
}

// numTx is the maximum number of tx to include (used for troubleshooting). default 0 (all transactions)
func SimulateBlock(block *types.Block, maxTx int, debug bool) (*ethrpc.FlashbotsCallBundleResponse, error) {
	if debug {
		fmt.Printf("Simulating block %s 0x%x %s \t %d tx \t timestamp: %d\n", block.Number(), block.Number(), block.Header().Hash(), len(block.Transactions()), block.Header().Time)
	}

	txs := make([]string, 0)
	for _, tx := range block.Transactions() {
		// fmt.Println("tx", i, tx.Hash(), "type", tx.Type())

		rlp := fbcommon.TxToRlp(tx)

		// Might need to strip beginning bytes
		if rlp[:2] == "b9" {
			rlp = rlp[6:]
		} else if rlp[:2] == "b8" {
			rlp = rlp[4:]
		}

		// callBundle expects a 0x prefix
		rlp = "0x" + rlp
		txs = append(txs, rlp)

		if maxTx > 0 && len(txs) == maxTx {
			break
		}
	}

	if debug {
		fmt.Printf("sending %d tx for simulation to %s...\n", len(txs), mevGethNode)
	}

	param := ethrpc.FlashbotsCallBundleParam{
		Txs:              txs,
		BlockNumber:      fmt.Sprintf("0x%x", block.Number()),
		StateBlockNumber: block.ParentHash().Hex(),
		GasLimit:         block.GasLimit(),
		Difficulty:       block.Difficulty().Uint64(),
		BaseFee:          block.BaseFee().Uint64(),
		// BlockNumber:      fmt.Sprintf("0x%x", block.Number()),
		// StateBlockNumber: fmt.Sprintf("0x%x", block.NumberU64()-1),
	}

	rpcClient := ethrpc.New(mevGethNode)
	rpcClient.Debug = debug

	privateKey, _ := crypto.GenerateKey()
	result, err := rpcClient.FlashbotsCallBundle(privateKey, param)
	if err != nil {
		return nil, err
	}

	// fmt.Println(result)
	// fmt.Println("Coinbase diff:", result.CoinbaseDiff)
	return &result, nil
}

// One tx breaks the simulation. Find the tx.
func FindBreakingTx(block *types.Block) (breakingTxIndex int) {
	numTransactions := len(block.Transactions()) // on first run, include all transactions
	if numTransactions == 0 {
		return -1
	}

	isFirst := true
	isSecond := false
	// lastBroken := 0
	lastStepSize := numTransactions
	lastWorking := 0

	for {
		fmt.Println("")

		// fmt.Println("\ntrying num tx:", numTransactions)
		_, err := SimulateBlock(block, numTransactions, true)
		hasError := err != nil

		// fmt.Println(numTransactions, isFirst, hasError, err)
		if isFirst { // first try: all
			isFirst = false
			isSecond = true

			if hasError {
				fmt.Println(err)
				numTransactions = 1
				continue
			} else {
				return -1 // block works as whole!
			}
		} else if isSecond { // second try: only 1 tx
			if hasError {
				fmt.Println(err)
				return 0
			}
			isSecond = false
			lastWorking = numTransactions
			numTransactions = len(block.Transactions()) / 2
			lastStepSize /= 2
			continue
		}

		if lastStepSize > 1 {
			lastStepSize /= 2
		}

		if hasError {
			// fmt.Println(numTransactions, "stepsize", lastStepSize, err)
			fmt.Println(err)
			if numTransactions < 2 {
				return 0
			}

			if lastWorking == numTransactions-1 {
				return numTransactions - 1
			}

			// try with fewer transactrions
			// lastBroken = numTransactions
			numTransactions -= lastStepSize
			continue
		}

		// no error. try with more
		lastWorking = numTransactions
		numTransactions += lastStepSize
	}
}
