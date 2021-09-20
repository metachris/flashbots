package common

import (
	"bytes"
	"context"
	"errors"
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
)

func StrToBigInt(s string) *big.Int {
	i := new(big.Int)
	i.SetString(s, 10)
	return i
}

func BigFloatToEString(f *big.Float, prec int) string {
	s1 := f.Text('f', 0)
	if len(s1) >= 16 {
		f2 := new(big.Float).Quo(f, big.NewFloat(1e18))
		s := f2.Text('f', prec)
		return s + "e+18"
	} else if len(s1) >= 9 {
		f2 := new(big.Float).Quo(f, big.NewFloat(1e9))
		s := f2.Text('f', prec)
		return s + "e+09"
	}
	return f.Text('f', prec)
}

func BigIntToEString(i *big.Int, prec int) string {
	f := new(big.Float)
	f.SetInt(i)
	s1 := f.Text('f', 0)
	if len(s1) < 9 {
		return i.String()
	}
	return BigFloatToEString(f, prec)
}

func TimeStringToSec(s string) (timespanSec int, err error) {
	isNegativeNumber := strings.HasPrefix(s, "-")
	if isNegativeNumber {
		s = s[1:]
	}

	var sec int = 0
	switch {
	case strings.HasSuffix(s, "s"):
		sec, err = strconv.Atoi(strings.TrimSuffix(s, "s"))
	case strings.HasSuffix(s, "m"):
		sec, err = strconv.Atoi(strings.TrimSuffix(s, "m"))
		sec *= 60
	case strings.HasSuffix(s, "h"):
		sec, err = strconv.Atoi(strings.TrimSuffix(s, "h"))
		sec *= 60 * 60
	case strings.HasSuffix(s, "d"):
		sec, err = strconv.Atoi(strings.TrimSuffix(s, "d"))
		sec *= 60 * 60 * 24
	default:
		err = errors.New("couldn't detect time format")
	}
	if isNegativeNumber {
		sec *= -1 // make negative
	}

	return sec, err
}

func TxToRlp(tx *types.Transaction) string {
	var buff bytes.Buffer
	tx.EncodeRLP(&buff)
	return fmt.Sprintf("%x", buff.Bytes())
}

func BlockToRlp(block *types.Block) string {
	var buff bytes.Buffer
	block.EncodeRLP(&buff)
	return fmt.Sprintf("%x", buff.Bytes())
}

func EnvStr(key string, defaultvalue string) string {
	res := os.Getenv(key)
	if res != "" {
		return res
	}
	return defaultvalue
}

func GetBlocks(blockChan chan<- *types.Block, client *ethclient.Client, startBlock int64, endBlock int64, concurrency int) {
	var blockWorkerWg sync.WaitGroup
	blockHeightChan := make(chan int64, 100) // blockHeight to fetch with receipts

	// Start eth client thread pool
	for w := 1; w <= concurrency; w++ {
		blockWorkerWg.Add(1)

		// Worker gets a block height from blockHeightChan, downloads it, and puts it in the blockChan
		go func() {
			defer blockWorkerWg.Done()
			for blockHeight := range blockHeightChan {
				// fmt.Println(blockHeight)
				block, err := client.BlockByNumber(context.Background(), big.NewInt(blockHeight))
				if err != nil {
					log.Println("Error getting block:", blockHeight, err)
					continue
				}
				blockChan <- block
			}
		}()
	}

	// Push blocks into channel, for workers to pick up
	for currentBlockNumber := startBlock; currentBlockNumber <= endBlock; currentBlockNumber++ {
		blockHeightChan <- currentBlockNumber
	}

	// Close worker channel and wait for workers to finish
	close(blockHeightChan)
	blockWorkerWg.Wait()
}

func PrintBlock(block *types.Block) {
	t := time.Unix(int64(block.Header().Time), 0).UTC()
	unclesStr := ""
	if len(block.Uncles()) > 0 {
		unclesStr = fmt.Sprintf("uncles=%d", len(block.Uncles()))
	}
	fmt.Printf("Block %d %s \t miner: %s \t %s \t tx=%-4d \t gas=%d \t %s\n", block.NumberU64(), block.Hash(), block.Coinbase(), t, len(block.Transactions()), block.GasUsed(), unclesStr)
}

var ColorGreen = "\033[1;32m%s\033[0m"
