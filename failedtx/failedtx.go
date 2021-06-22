// Representation of a failed Flashbots or other 0-gas transaction (used in webserver)
package failedtx

import "fmt"

// FailedTx contains information about a failed 0-gas or Flashbots tx
type FailedTx struct {
	Hash        string
	From        string
	To          string
	Block       uint64
	IsFlashbots bool // if false then it's a failed 0-gas tx but not from Flashbots
}

// BlockWithFailedTx is used by the webserver, which returns the last 100 blocks with failed tx
type BlockWithFailedTx struct {
	BlockHeight int64
	FailedTx    []FailedTx
}

func MsgForFailedTx(tx FailedTx) string {
	if tx.IsFlashbots {
		return fmt.Sprintf("Failed Flashbots tx [%s](https://etherscan.io/tx/%s) from [%s](https://etherscan.io/address/%s) in block [%d](https://etherscan.io/block/%d)\n", tx.Hash, tx.Hash, tx.From, tx.From, tx.Block, tx.Block)
	} else {
		return fmt.Sprintf("Failed 0-gas tx [%s](https://etherscan.io/tx/%s) from [%s](https://etherscan.io/address/%s) in block [%d](https://etherscan.io/block/%d)\n", tx.Hash, tx.Hash, tx.From, tx.From, tx.Block, tx.Block)
	}
}
