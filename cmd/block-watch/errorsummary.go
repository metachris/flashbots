package main

import (
	"fmt"
	"sort"
	"time"

	"github.com/metachris/flashbots/blockcheck"
)

type ErrorSummary struct {
	TimeStarted time.Time
	MinerErrors map[string]*blockcheck.MinerErrors
}

func NewErrorSummary() ErrorSummary {
	return ErrorSummary{
		TimeStarted: time.Now(),
		MinerErrors: make(map[string]*blockcheck.MinerErrors),
	}
}

func (es *ErrorSummary) String() (ret string) {
	// Get list of keys by number of errorBlocks
	keys := make([]string, 0, len(es.MinerErrors))
	for k := range es.MinerErrors {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return len(es.MinerErrors[keys[i]].Blocks) > len(es.MinerErrors[keys[j]].Blocks)
	})

	for _, key := range keys {
		minerErrors := es.MinerErrors[key]
		minerId := key
		if minerErrors.MinerName != "" {
			minerId += fmt.Sprintf(" (%s)", minerErrors.MinerName)
		}
		ret += fmt.Sprintf("%-66s errorBlocks=%d \t failed0gas=%d \t failedFbTx=%d \t bundlePaysMore=%d \t bundleTooLowFee=%d \t has0fee=%d \t hasNegativeFee=%d\n", minerId, len(minerErrors.Blocks), minerErrors.ErrorCounts.Failed0GasTx, minerErrors.ErrorCounts.FailedFlashbotsTx, minerErrors.ErrorCounts.BundlePaysMoreThanPrevBundle, minerErrors.ErrorCounts.BundleHasLowerFeeThanLowestNonFbTx, minerErrors.ErrorCounts.BundleHas0Fee, minerErrors.ErrorCounts.BundleHasNegativeFee)
	}
	return ret
}

func (es *ErrorSummary) AddErrorCounts(MinerHash string, MinerName string, block int64, errors blockcheck.ErrorCounts) {
	_, found := es.MinerErrors[MinerHash]
	if !found {
		es.MinerErrors[MinerHash] = &blockcheck.MinerErrors{
			MinerHash: MinerHash,
			MinerName: MinerName,
			Blocks:    make(map[int64]bool),
		}
	}

	es.MinerErrors[MinerHash].AddErrorCounts(block, errors)
	if es.TimeStarted == time.Unix(0, 0) {
		es.TimeStarted = time.Now()
	}
}

func (es *ErrorSummary) AddCheckErrors(check *blockcheck.BlockCheck) {
	es.AddErrorCounts(check.Miner, check.MinerName, check.Number, check.ErrorCounter)
}

func (es *ErrorSummary) Reset() {
	es.TimeStarted = time.Now()
	es.MinerErrors = make(map[string]*blockcheck.MinerErrors)
}
