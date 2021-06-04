package main

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/metachris/go-ethutils/utils"
)

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
				days, _ := strconv.Atoi(date[1 : len(date)-1])
				t := time.Now().AddDate(0, 0, -days) // todo
				startTime = t.Truncate(24 * time.Hour)
			} else if strings.HasSuffix(date, "m") {
				months, _ := strconv.Atoi(date[1 : len(date)-1])
				t := time.Now().AddDate(0, -months, 0)
				startTime = t.Truncate(24 * time.Hour)
			} else if strings.HasSuffix(date, "y") {
				years, _ := strconv.Atoi(date[1 : len(date)-1])
				t := time.Now().AddDate(-years, 0, 0)
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
		endBlockHeader, err := utils.GetFirstBlockHeaderAtOrAfterTime(client, endTime)
		utils.Perror(err)
		endBlock = endBlockHeader.Number.Int64() - 1
	} else {
		panic("No valid block range")
	}

	if endBlock < startBlock {
		endBlock = startBlock
	}

	return startBlock, endBlock
}
