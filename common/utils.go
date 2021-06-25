package common

import (
	"fmt"
	"math/big"

	"github.com/metachris/go-ethutils/utils"
)

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

func SprintBlock(b *Block, color bool, markdown bool) (msg string) {
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

		msg += fmt.Sprintf("- bundle %d: tx: %d, gasUsed: %7d \t coinbase_transfer: %11v, total_miner_reward: %11v \t coinbase/gasused: %13v, reward/gasused: %13v %v", bundle.Index, len(bundle.Transactions), bundle.TotalGasUsed, BigIntToEString(bundle.TotalCoinbaseTransfer, 4), BigIntToEString(bundle.TotalMinerReward, 4), BigIntToEString(bundle.CoinbaseDivGasUsed, 4), BigIntToEString(bundle.RewardDivGasUsed, 4), percentPart)
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
