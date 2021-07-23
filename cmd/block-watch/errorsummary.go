package main

import (
	"fmt"
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
	for k, v := range es.MinerErrors {
		minerInfo := k
		if v.MinerName != "" {
			minerInfo += fmt.Sprintf(" (%s)", v.MinerName)
		}
		ret += fmt.Sprintf("%-66s errorBlocks=%d \t failed0gas=%d \t failedFbTx=%d \t bundlePaysMore=%d \t bundleTooLowFee=%d \t has0fee=%d \t hasNegativeFee=%d\n", minerInfo, len(v.Blocks), v.ErrorCounts.Failed0GasTx, v.ErrorCounts.FailedFlashbotsTx, v.ErrorCounts.BundlePaysMoreThanPrevBundle, v.ErrorCounts.BundleHasLowerFeeThanLowestNonFbTx, v.ErrorCounts.BundleHas0Fee, v.ErrorCounts.BundleHasNegativeFee)
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
