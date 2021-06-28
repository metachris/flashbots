// Representation of a failed Flashbots or other 0-gas transaction (used in webserver)
package blockcheck

// FailedTx contains information about a failed 0-gas or Flashbots tx
type FailedTx struct {
	Hash        string
	IsFlashbots bool
	From        string
	To          string
	Block       uint64
}

// // BlockWithFailedTx is used by the webserver, which returns the last 100 blocks with failed tx
// type BlockWithFailedTx struct {
// 	BlockHeight int64
// 	FailedTx    []FailedTx
// }

// func MsgForFailedTx(tx FailedTx, long bool) (msg string) {
// 	if tx.IsFlashbots {
// 		msg += fmt.Sprintf("Failed Flashbots tx [%s](<https://etherscan.io/tx/%s>) from [%s](<https://etherscan.io/address/%s>)", tx.Hash, tx.Hash, tx.From, tx.From)
// 	} else {
// 		msg += fmt.Sprintf("Failed 0-gas tx [%s](<https://etherscan.io/tx/%s>) from [%s](<https://etherscan.io/address/%s>)", tx.Hash, tx.Hash, tx.From, tx.From)
// 	}

// 	if long {
// 		msg += fmt.Sprintf(" in block [%d](<https://etherscan.io/block/%d>) (miner: [%s](<https://etherscan.io/address/%s>))\n", tx.Block, tx.Block, tx.Miner, tx.Miner)
// 	} else {
// 		msg += "\n"
// 	}

// 	return msg
// }
