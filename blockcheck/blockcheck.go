package blockcheck

import (
	"errors"
	"fmt"
	"math/big"
	"sort"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/metachris/flashbots/api"
	"github.com/metachris/flashbots/common"
	"github.com/metachris/go-ethutils/blockswithtx"
	"github.com/metachris/go-ethutils/utils"
)

var (
	ErrFlashbotsApiDoesntHaveThatBlockYet = errors.New("flashbots API latest height < requested block height")
)

var ThresholdBiggestBundlePercentPriceDiff float32 = 50
var ThresholdBundleIsPayingLessThanLowestTxPercentDiff float32 = 50

type BlockCheck struct {
	Number int64
	Miner  string

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

	HasFailedFlashbotsTx bool
	HasFailed0GasTx      bool
}

func CheckBlock(blockWithTx *blockswithtx.BlockWithTxReceipts) (blockCheck *BlockCheck, err error) {
	check := BlockCheck{
		BlockWithTxReceipts:   blockWithTx,
		EthBlock:              blockWithTx.Block,
		FlashbotsTransactions: make([]api.FlashbotsTransaction, 0),

		Number:  blockWithTx.Block.Number().Int64(),
		Miner:   blockWithTx.Block.Coinbase().Hex(),
		Bundles: make([]*common.Bundle, 0),
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
	// Bundle percent price diff
	if b.BiggestBundlePercentPriceDiff >= ThresholdBiggestBundlePercentPriceDiff {
		return true
	}

	// Bundle lower than lowest non-fb tx
	if b.BundleIsPayingLessThanLowestTxPercentDiff >= ThresholdBundleIsPayingLessThanLowestTxPercentDiff {
		return true
	}

	// Failed tx
	if len(b.FailedTx) > 0 {
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

			if lastCoinbaseDivGasused.Int64() != 0 &&
				bundle.CoinbaseDivGasUsed.Cmp(lastCoinbaseDivGasused) == 1 &&
				bundle.RewardDivGasUsed.Cmp(lastRewardDivGasused) == 1 &&
				bundle.CoinbaseDivGasUsed.Cmp(lastRewardDivGasused) == 1 {

				msg := fmt.Sprintf("bundle %d pays %v%s more than previous bundle\n", bundle.Index, percentDiff.Text('f', 2), "%")
				b.AddError(msg)
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
	// lowestGasPriceTxHash := ""
	for _, tx := range b.EthBlock.Transactions() {
		isFlashbotsTx := b.IsFlashbotsTx(tx.Hash().String())
		if isFlashbotsTx {
			continue
		}

		if lowestGasPrice.Int64() == -1 || tx.GasPrice().Cmp(lowestGasPrice) == -1 {
			lowestGasPrice = tx.GasPrice()
			// lowestGasPriceTxHash = tx.Hash().Hex()
		}
	}

	// step 2. compare all fb-tx effective gas prices
	for _, bundle := range b.Bundles {
		if bundle.RewardDivGasUsed.Cmp(lowestGasPrice) == -1 {
			// calculate percent difference:
			fCur := new(big.Float).SetInt(bundle.RewardDivGasUsed)
			fLow := new(big.Float).SetInt(lowestGasPrice)
			diffPercent1 := new(big.Float).Quo(fCur, fLow)
			diffPercent2 := new(big.Float).Sub(big.NewFloat(1), diffPercent1)
			diffPercent := new(big.Float).Mul(diffPercent2, big.NewFloat(100))

			msg := fmt.Sprintf("bundle %d has %s%s lower effective-gas-price (%v) than lowest non-fb transaction (%v)\n", bundle.Index, diffPercent.Text('f', 2), "%", common.BigIntToEString(bundle.RewardDivGasUsed, 4), common.BigIntToEString(lowestGasPrice, 4))
			b.AddError(msg)
			bundle.IsPayingLessThanLowestTx = true
			b.BundleIsPayingLessThanLowestTxPercentDiff, _ = diffPercent.Float32()
		}
	}
}

func (b *BlockCheck) Sprint(color bool, markdown bool) (msg string) {
	// Print block info
	// minerAddr, found := AddressLookupService.GetAddressDetail(b.Miner)
	if markdown {
		msg = fmt.Sprintf("Block [%d](<https://etherscan.io/block/%d>) ([bundle-explorer](<https://flashbots-explorer.marto.lol/?block=%d>)), miner [%s](<https://etherscan.io/address/%s>) - tx: %d, fb-tx: %d, bundles: %d\n", b.Number, b.Number, b.Number, b.Miner, b.Miner, len(b.BlockWithTxReceipts.Block.Transactions()), len(b.FlashbotsApiBlock.Transactions), len(b.Bundles))
	} else {
		msg = fmt.Sprintf("Block %d, miner %s - tx: %d, fb-tx: %d, bundles: %d\n", b.Number, b.Miner, len(b.BlockWithTxReceipts.Block.Transactions()), len(b.FlashbotsTransactions), len(b.Bundles))
	}

	// Print errors
	for _, err := range b.Errors {
		err = "- error: " + err
		if color {
			msg += fmt.Sprintf(utils.WarningColor, err)
		} else {
			msg += err
		}
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
			msg := fmt.Sprintf("failed Flashbots tx [%s](<https://etherscan.io/tx/%s>) from [%s](<https://etherscan.io/address/%s>)\n", fbTx.Hash, fbTx.Hash, fbTx.EoaAddress, fbTx.EoaAddress)
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
			if receipt.Status == 0 { // successful tx
				if _, exists := b.FailedTx[tx.Hash().String()]; exists {
					// Already known (Flashbots TX)
					continue
				}

				from, _ := utils.GetTxSender(tx)
				b.FailedTx[tx.Hash().String()] = &FailedTx{
					Hash:        tx.Hash().String(),
					IsFlashbots: false,
					From:        from.String(),
					To:          tx.To().String(),
					Block:       uint64(b.Number),
				}

				msg := fmt.Sprintf("failed 0-gas tx [%s](<https://etherscan.io/tx/%s>) from [%s](<https://etherscan.io/address/%s>)\n", tx.Hash(), tx.Hash(), from, from)
				b.AddError(msg)
				b.HasFailed0GasTx = true
			}
		}
	}

	return failedTransactions
}
