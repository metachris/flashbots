package blockcheck

import (
	"fmt"
	"math/big"
	"sort"

	"github.com/metachris/flashbots/api"
	"github.com/metachris/flashbots/common"
	"github.com/metachris/go-ethutils/utils"
)

type Block struct {
	Number  int64
	Miner   string
	Bundles []*common.Bundle

	Errors                        []string
	BiggestBundlePercentPriceDiff float32 // on order error, % difference to previous bundle
}

// AddBundle adds the bundle and sorts them by Index
func (b *Block) AddBundle(bundle *common.Bundle) {
	b.Bundles = append(b.Bundles, bundle)

	// Bring bundles into order
	sort.SliceStable(b.Bundles, func(i, j int) bool {
		return b.Bundles[i].Index < b.Bundles[j].Index
	})
}

func (b *Block) AddError(msg string) {
	b.Errors = append(b.Errors, msg)
}

func (b *Block) HasErrors() bool {
	return len(b.Errors) > 0
}

func NewBlockFromApiBlock(apiBlock api.FlashbotsBlock) *Block {
	block := Block{Number: apiBlock.BlockNumber, Miner: apiBlock.Miner}

	// Create the bundles
	bundles := make(map[int64]*common.Bundle)
	for _, tx := range apiBlock.Transactions {
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
		block.AddBundle(bundle)
	}

	return &block
}

// Check analyzes the Flashbots bundles and adds errors when issues are found
func (block *Block) Check() {
	numBundles := len(block.Bundles)

	// Check 1: do all bundles exists or are there gaps?
	for i := 0; i < numBundles; i++ {
		if block.Bundles[int64(i)] == nil {
			block.AddError(fmt.Sprintf("- error: missing bundle # %d in block %d", i, block.Number))
		}
	}

	// Check 2: are the bundles in the correct order?
	lastCoinbaseDivGasused := new(big.Int)
	lastRewardDivGasused := new(big.Int)
	for i := 0; i < numBundles; i++ {
		bundle := block.Bundles[int64(i)]

		// if not first bundle, and value larger than from last bundle, print the error
		if lastCoinbaseDivGasused.Int64() == 0 {
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

				msg := fmt.Sprintf("- order error: bundle %d pays %v%% more than previous bundle\n", bundle.Index, percentDiff.Text('f', 2))
				block.AddError(msg)
				bundle.IsOutOfOrder = true
				diffFloat, _ := percentDiff.Float32()
				if diffFloat > block.BiggestBundlePercentPriceDiff {
					block.BiggestBundlePercentPriceDiff = diffFloat
				}
			}
		}

		lastCoinbaseDivGasused = bundle.CoinbaseDivGasUsed
		lastRewardDivGasused = bundle.RewardDivGasUsed
	}
}

func (b *Block) Sprint(color bool, markdown bool) (msg string) {
	// Print block info
	// minerAddr, found := AddressLookupService.GetAddressDetail(b.Miner)
	if markdown {
		msg = fmt.Sprintf("Block [%d](<https://etherscan.io/block/%d>) ([bundle-explorer](<https://flashbots-explorer.marto.lol/?block=%d>)) - miner: [%s](<https://etherscan.io/address/%s>), bundles: %d\n", b.Number, b.Number, b.Number, b.Miner, b.Miner, len(b.Bundles))
	} else {
		msg = fmt.Sprintf("Block %d - miner: %s, bundles: %d\n", b.Number, b.Miner, len(b.Bundles))
	}

	// Print errors
	for _, err := range b.Errors {
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
			percentPart = fmt.Sprintf("(%6s%%)", bundle.PercentPriceDiff.Text('f', 2))
		} else if bundle.PercentPriceDiff.Cmp(big.NewFloat(0)) == 1 {
			percentPart = fmt.Sprintf("(+%5s%%)", bundle.PercentPriceDiff.Text('f', 2))
		}

		msg += fmt.Sprintf("- bundle %d: tx: %d, gasUsed: %7d \t coinbase_transfer: %11v, total_miner_reward: %11v \t coinbase/gasused: %13v, reward/gasused: %13v %v", bundle.Index, len(bundle.Transactions), bundle.TotalGasUsed, common.BigIntToEString(bundle.TotalCoinbaseTransfer, 4), common.BigIntToEString(bundle.TotalMinerReward, 4), common.BigIntToEString(bundle.CoinbaseDivGasUsed, 4), common.BigIntToEString(bundle.RewardDivGasUsed, 4), percentPart)
		if bundle.IsOutOfOrder {
			msg += " <--- out_of_order"
		}
		msg += "\n"
	}

	if markdown {
		msg += "```"
	}

	return msg
}
