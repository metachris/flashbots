package blockcheck

import (
	"errors"
	"fmt"
	"math/big"
	"sort"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/metachris/flashbots/api"
	"github.com/metachris/flashbots/common"
	"github.com/metachris/go-ethutils/addresslookup"
	"github.com/metachris/go-ethutils/blockswithtx"
	"github.com/metachris/go-ethutils/utils"
)

var (
	ErrFlashbotsApiDoesntHaveThatBlockYet = errors.New("flashbots API latest height < requested block height")
)

var ThresholdBiggestBundlePercentPriceDiff float32 = 50
var ThresholdBundleIsPayingLessThanLowestTxPercentDiff float32 = 50

var AddressLookup *addresslookup.AddressLookupService
var AddressesUpdated time.Time

type ErrorCounts struct {
	FailedFlashbotsTx                  uint64
	Failed0GasTx                       uint64
	BundlePaysMoreThanPrevBundle       uint64
	BundleHasLowerFeeThanLowestNonFbTx uint64
	BundleHas0Fee                      uint64
	BundleHasNegativeFee               uint64
}

func (ec *ErrorCounts) Add(counts ErrorCounts) {
	ec.FailedFlashbotsTx += counts.FailedFlashbotsTx
	ec.Failed0GasTx += counts.Failed0GasTx
	ec.BundlePaysMoreThanPrevBundle += counts.BundlePaysMoreThanPrevBundle
	ec.BundleHasLowerFeeThanLowestNonFbTx += counts.BundleHasLowerFeeThanLowestNonFbTx
	ec.BundleHas0Fee += counts.BundleHas0Fee
	ec.BundleHasNegativeFee += counts.BundleHasNegativeFee
}

type BlockCheck struct {
	Number    int64
	Miner     string
	MinerName string

	BlockWithTxReceipts   *blockswithtx.BlockWithTxReceipts
	EthBlock              *types.Block
	FlashbotsApiBlock     *api.FlashbotsBlock
	FlashbotsTransactions []api.FlashbotsTransaction
	Bundles               []*common.Bundle

	// Collection of errors
	Errors   []string
	FailedTx map[string]*FailedTx

	// Helpers to filter later in user code
	BiggestBundlePercentPriceDiff             float32 // on order error, max % difference to previous bundle
	BundleIsPayingLessThanLowestTxPercentDiff float32

	HasBundleWith0EffectiveGasPrice bool
	HasFailedFlashbotsTx            bool
	HasFailed0GasTx                 bool
	ManualHasSeriousError           bool // manually set by specific error conditions

	ErrorCounter ErrorCounts
}

func CheckBlock(blockWithTx *blockswithtx.BlockWithTxReceipts) (blockCheck *BlockCheck, err error) {
	// Init / update AddressLookup service
	if AddressLookup == nil {
		AddressLookup = addresslookup.NewAddressLookupService(nil)
		err = AddressLookup.AddAllAddresses()
		if err != nil {
			return blockCheck, err
		}
		AddressesUpdated = time.Now()
	} else { // after 5 minutes, update addresses
		timeSinceAddressUpdate := time.Since(AddressesUpdated)
		if timeSinceAddressUpdate.Seconds() > 60*5 {
			AddressLookup.ClearCache()
			AddressLookup.AddAllAddresses()
			AddressesUpdated = time.Now()
		}
	}

	// Lookup miner details
	minerAddr, _ := AddressLookup.GetAddressDetail(blockWithTx.Block.Coinbase().Hex())

	// Create check result
	check := BlockCheck{
		BlockWithTxReceipts:   blockWithTx,
		EthBlock:              blockWithTx.Block,
		FlashbotsTransactions: make([]api.FlashbotsTransaction, 0),

		Number:       blockWithTx.Block.Number().Int64(),
		Miner:        blockWithTx.Block.Coinbase().Hex(),
		MinerName:    minerAddr.Name,
		Bundles:      make([]*common.Bundle, 0),
		ErrorCounter: ErrorCounts{},
	}

	err = check.QueryFlashbotsApi()
	if err != nil {
		return blockCheck, err
	}
	check.CreateBundles()
	check.Check()
	return &check, nil
}

func (b *BlockCheck) AddError(msg string) {
	b.Errors = append(b.Errors, msg)
}

func (b *BlockCheck) HasErrors() bool {
	return len(b.Errors) > 0
}

func (b *BlockCheck) HasSeriousErrors() bool {
	// Failed tx
	if len(b.FailedTx) > 0 {
		return true
	}

	if b.ManualHasSeriousError {
		return true
	}

	// Bundle percent price diff
	if b.BiggestBundlePercentPriceDiff >= ThresholdBiggestBundlePercentPriceDiff {
		return true
	}

	// Bundle lower than lowest non-fb tx
	if b.BundleIsPayingLessThanLowestTxPercentDiff >= ThresholdBundleIsPayingLessThanLowestTxPercentDiff {
		return true
	}

	return false
}

func (b *BlockCheck) HasLessSeriousErrors() bool {
	// Failed tx
	if len(b.FailedTx) > 0 {
		return true
	}

	// Bundle percent price diff
	if b.BiggestBundlePercentPriceDiff >= 25 {
		return true
	}

	// Bundle lower than lowest non-fb tx
	if b.BundleIsPayingLessThanLowestTxPercentDiff >= 25 {
		return true
	}

	return false
}

// AddBundle adds the bundle and sorts them by Index
func (b *BlockCheck) AddBundle(bundle *common.Bundle) {
	b.Bundles = append(b.Bundles, bundle)

	// Bring bundles into order
	sort.SliceStable(b.Bundles, func(i, j int) bool {
		return b.Bundles[i].Index < b.Bundles[j].Index
	})
}

func (b *BlockCheck) QueryFlashbotsApi() error {
	// API call to flashbots
	opts := api.GetBlocksOptions{BlockNumber: b.Number}
	flashbotsResponse, err := api.GetBlocks(&opts)
	if err != nil {
		return err
	}

	// Return an error if API doesn't have the block yet
	if flashbotsResponse.LatestBlockNumber < b.Number {
		return ErrFlashbotsApiDoesntHaveThatBlockYet
	}

	if len(flashbotsResponse.Blocks) != 1 {
		return nil
	}

	b.FlashbotsApiBlock = &flashbotsResponse.Blocks[0]
	b.FlashbotsTransactions = b.FlashbotsApiBlock.Transactions
	b.CreateBundles()

	return nil
}

func (b *BlockCheck) CreateBundles() {
	if b.FlashbotsApiBlock == nil {
		return
	}

	// Clear old bundles
	b.Bundles = make([]*common.Bundle, 0)

	// Create the bundles from all Flashbots transactions in this block
	bundles := make(map[int64]*common.Bundle)
	for _, tx := range b.FlashbotsApiBlock.Transactions {
		bundleIndex := tx.BundleIndex
		bundle, exists := bundles[bundleIndex]
		if !exists {
			bundle = common.NewBundle()
			bundle.Index = bundleIndex
			bundles[bundleIndex] = bundle
		}

		// Update bundle information
		bundle.Transactions = append(bundle.Transactions, tx)

		txMinerReward := new(big.Int)
		txMinerReward.SetString(tx.TotalMinerReward, 10)

		txCoinbaseTransfer := new(big.Int)
		txCoinbaseTransfer.SetString(tx.CoinbaseTransfer, 10)

		txGasUsed := big.NewInt(tx.GasUsed)

		bundle.TotalMinerReward = new(big.Int).Add(bundle.TotalMinerReward, txMinerReward)
		bundle.TotalCoinbaseTransfer = new(big.Int).Add(bundle.TotalCoinbaseTransfer, txCoinbaseTransfer)
		bundle.TotalGasUsed = new(big.Int).Add(bundle.TotalGasUsed, txGasUsed)

		bundle.CoinbaseDivGasUsed = new(big.Int).Div(bundle.TotalCoinbaseTransfer, bundle.TotalGasUsed)
		bundle.RewardDivGasUsed = new(big.Int).Div(bundle.TotalMinerReward, bundle.TotalGasUsed)
	}

	// Add bundles to the block
	for _, bundle := range bundles {
		b.AddBundle(bundle)
	}
}

func (b *BlockCheck) IsFlashbotsTx(hash string) bool {
	for _, tx := range b.FlashbotsTransactions {
		if tx.Hash == hash {
			return true
		}
	}
	return false
}

// Check analyzes the Flashbots bundles and adds errors when issues are found
func (b *BlockCheck) Check() {
	numBundles := len(b.Bundles)

	// TODO: Check uncle-bandit-attack
	// b.BlockWithTxReceipts.Block.Uncles()

	// Check 0: contains failed Flashbots or 0-gas tx
	b.checkBlockForFailedTx()

	// Check 1: do all bundles exists or are there gaps?
	for i := 0; i < numBundles; i++ {
		if b.Bundles[int64(i)] == nil {
			b.AddError(fmt.Sprintf("- error: missing bundle # %d in block %d", i, b.Number))
		}
	}

	// Check 2: are the bundles in the correct order?
	lastCoinbaseDivGasused := big.NewInt(-1)
	lastRewardDivGasused := big.NewInt(-1)
	for i := 0; i < numBundles; i++ {
		bundle := b.Bundles[int64(i)]

		// if not first bundle, and value larger than from last bundle, print the error
		if lastCoinbaseDivGasused.Int64() == -1 {
			// nothing to do on the first bundle
		} else {
			percentDiff := new(big.Float).Quo(new(big.Float).SetInt(bundle.RewardDivGasUsed), new(big.Float).SetInt(lastRewardDivGasused))
			percentDiff = new(big.Float).Sub(percentDiff, big.NewFloat(1))
			percentDiff = new(big.Float).Mul(percentDiff, big.NewFloat(100))
			bundle.PercentPriceDiff = percentDiff

			if bundle.CoinbaseDivGasUsed.Cmp(lastCoinbaseDivGasused) == 1 &&
				bundle.RewardDivGasUsed.Cmp(lastRewardDivGasused) == 1 &&
				bundle.CoinbaseDivGasUsed.Cmp(lastRewardDivGasused) == 1 {

				msg := fmt.Sprintf("bundle %d pays %v%s more than previous bundle\n", bundle.Index, percentDiff.Text('f', 2), "%")
				b.AddError(msg)
				b.ErrorCounter.BundlePaysMoreThanPrevBundle += 1
				bundle.IsOutOfOrder = true
				diffFloat, _ := percentDiff.Float32()
				if diffFloat > b.BiggestBundlePercentPriceDiff {
					b.BiggestBundlePercentPriceDiff = diffFloat
				}
			}
		}

		lastCoinbaseDivGasused = bundle.CoinbaseDivGasUsed
		lastRewardDivGasused = bundle.RewardDivGasUsed
	}

	// Check 3: bundle effective gas price > lowest tx gas price
	// step 1. find lowest non-fb-tx gas price
	lowestGasPrice := big.NewInt(-1)
	lowestGasPriceTxHash := ""
	for _, tx := range b.EthBlock.Transactions() {
		isFlashbotsTx := b.IsFlashbotsTx(tx.Hash().String())
		if isFlashbotsTx {
			continue
		}

		if lowestGasPrice.Int64() == -1 || tx.GasPrice().Cmp(lowestGasPrice) == -1 {
			if utils.IsBigIntZero(tx.GasPrice()) && len(tx.Data()) > 0 { // don't count Flashbots-like tx
				continue
			}
			lowestGasPrice = tx.GasPrice()
			lowestGasPriceTxHash = tx.Hash().Hex()
		}
	}

	// step 2. check gas prices and fees
	for _, bundle := range b.Bundles {
		if bundle.RewardDivGasUsed.Cmp(ethcommon.Big0) == -1 { // negative fee
			bundle.IsNegativeEffectiveGasPrice = true
			msg := fmt.Sprintf("bundle %d has negative effective-gas-price (%v)\n", bundle.Index, common.BigIntToEString(bundle.RewardDivGasUsed, 4))
			b.AddError(msg)
			b.ErrorCounter.BundleHasNegativeFee += 1
			b.ManualHasSeriousError = true

		} else if utils.IsBigIntZero(bundle.RewardDivGasUsed) { // 0 fee
			bundle.Is0EffectiveGasPrice = true
			msg := fmt.Sprintf("bundle %d has 0 effective-gas-price\n", bundle.Index)
			b.AddError(msg)
			b.ErrorCounter.BundleHas0Fee += 1
			b.HasBundleWith0EffectiveGasPrice = true
			b.ManualHasSeriousError = true

		} else if bundle.RewardDivGasUsed.Cmp(lowestGasPrice) == -1 { // lower fee than lowest non-fb TX
			bundle.IsPayingLessThanLowestTx = true

			// calculate percent difference:
			fCur := new(big.Float).SetInt(bundle.RewardDivGasUsed)
			fLow := new(big.Float).SetInt(lowestGasPrice)
			diffPercent1 := new(big.Float).Quo(fCur, fLow)
			diffPercent2 := new(big.Float).Sub(big.NewFloat(1), diffPercent1)
			diffPercent := new(big.Float).Mul(diffPercent2, big.NewFloat(100))

			msg := fmt.Sprintf("bundle %d has %s%s lower effective-gas-price (%v) than [lowest non-fb transaction](<https://etherscan.io/tx/%s>) (%v)\n", bundle.Index, diffPercent.Text('f', 2), "%", common.BigIntToEString(bundle.RewardDivGasUsed, 4), lowestGasPriceTxHash, common.BigIntToEString(lowestGasPrice, 4))
			b.AddError(msg)
			b.ErrorCounter.BundleHasLowerFeeThanLowestNonFbTx += 1
			b.BundleIsPayingLessThanLowestTxPercentDiff, _ = diffPercent.Float32()
		}
	}
}

func (b *BlockCheck) SprintHeader(color bool, markdown bool) (msg string) {
	minerAddr, found := AddressLookup.GetAddressDetail(b.Miner)
	minerStr := fmt.Sprintf("[%s](<https://etherscan.io/address/%s>)", b.Miner, b.Miner)
	if found {
		minerStr = fmt.Sprintf("[%s](<https://etherscan.io/address/%s>)", minerAddr.Name, b.Miner)
	}

	if markdown {
		msg = fmt.Sprintf("Block [%d](<https://etherscan.io/block/%d>) ([bundle explorer](<https://flashbots-explorer.marto.lol/?block=%d>)), miner: %s - tx: %d, fb-tx: %d, bundles: %d", b.Number, b.Number, b.Number, minerStr, len(b.BlockWithTxReceipts.Block.Transactions()), len(b.FlashbotsApiBlock.Transactions), len(b.Bundles))
	} else {
		msg = fmt.Sprintf("Block %d, miner %s - tx: %d, fb-tx: %d, bundles: %d", b.Number, minerStr, len(b.BlockWithTxReceipts.Block.Transactions()), len(b.FlashbotsTransactions), len(b.Bundles))
	}
	return msg
}

func (b *BlockCheck) Sprint(color bool, markdown bool, includeBundles bool) (msg string) {
	msg = b.SprintHeader(color, markdown)
	msg += "\n"

	// Print errors
	for _, err := range b.Errors {
		err = "- error: " + err
		if color {
			msg += fmt.Sprintf(utils.WarningColor, err)
		} else {
			msg += err
		}
	}

	if !includeBundles {
		return msg
	}

	if markdown {
		msg += "```"
	}

	// Print bundles
	for _, bundle := range b.Bundles {
		// Build string for percent(gasprice difference to previous bundle)
		percentPart := ""
		if bundle.PercentPriceDiff.Cmp(big.NewFloat(0)) == -1 {
			percentPart = fmt.Sprintf("(%6s%s)", bundle.PercentPriceDiff.Text('f', 2), "%")
		} else if bundle.PercentPriceDiff.Cmp(big.NewFloat(0)) == 1 {
			percentPart = fmt.Sprintf("(+%5s%s)", bundle.PercentPriceDiff.Text('f', 2), "%")
		}

		msg += fmt.Sprintf("- bundle %d: tx: %d, gasUsed: %7d \t coinbase_transfer: %13v, total_miner_reward: %13v \t coinbase/gasused: %13v, reward/gasused: %13v %v", bundle.Index, len(bundle.Transactions), bundle.TotalGasUsed, common.BigIntToEString(bundle.TotalCoinbaseTransfer, 4), common.BigIntToEString(bundle.TotalMinerReward, 4), common.BigIntToEString(bundle.CoinbaseDivGasUsed, 4), common.BigIntToEString(bundle.RewardDivGasUsed, 4), percentPart)
		if bundle.IsOutOfOrder || bundle.IsPayingLessThanLowestTx {
			msg += " <--"
		}
		msg += "\n"
	}

	if markdown {
		msg += "```"
	}

	return msg
}

func (b *BlockCheck) checkBlockForFailedTx() (failedTransactions []FailedTx) {
	b.FailedTx = make(map[string]*FailedTx)

	// 1. iterate over all Flashbots transactions and check if any has failed
	for _, fbTx := range b.FlashbotsTransactions {
		receipt := b.BlockWithTxReceipts.TxReceipts[ethcommon.HexToHash(fbTx.Hash)]
		if receipt == nil {
			continue
		}

		if receipt.Status == 0 { // failed Flashbots TX
			b.FailedTx[fbTx.Hash] = &FailedTx{
				Hash:        fbTx.Hash,
				IsFlashbots: true,
				From:        fbTx.EoaAddress,
				To:          fbTx.ToAddress,
				Block:       uint64(fbTx.BlockNumber),
			}

			msg := fmt.Sprintf("failed Flashbots tx [%s](<https://etherscan.io/tx/%s>) in bundle %d (from [%s](<https://etherscan.io/address/%s>))\n", fbTx.Hash, fbTx.Hash, fbTx.BundleIndex, fbTx.EoaAddress, fbTx.EoaAddress)
			b.ErrorCounter.FailedFlashbotsTx += 1
			b.AddError(msg)
			b.HasFailedFlashbotsTx = true
		}
	}

	// 2. iterate over all failed 0-gas transactions in the EthBlock
	for _, tx := range b.EthBlock.Transactions() {
		receipt := b.BlockWithTxReceipts.TxReceipts[tx.Hash()]
		if receipt == nil {
			continue
		}

		if utils.IsBigIntZero(tx.GasPrice()) && len(tx.Data()) > 0 {
			if receipt.Status == 0 { // failed tx
				if _, exists := b.FailedTx[tx.Hash().String()]; exists {
					// Already known (Flashbots TX)
					continue
				}

				from, _ := utils.GetTxSender(tx)
				to := ""
				if tx.To() != nil {
					to = tx.To().String()
				}
				b.FailedTx[tx.Hash().String()] = &FailedTx{
					Hash:        tx.Hash().String(),
					IsFlashbots: false,
					From:        from.String(),
					To:          to,
					Block:       uint64(b.Number),
				}

				msg := fmt.Sprintf("failed 0-gas tx [%s](<https://etherscan.io/tx/%s>) from [%s](<https://etherscan.io/address/%s>)\n", tx.Hash(), tx.Hash(), from, from)
				b.AddError(msg)
				b.ErrorCounter.Failed0GasTx += 1
				b.HasFailed0GasTx = true
			}
		}
	}

	return failedTransactions
}
