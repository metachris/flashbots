package flashbotsfailedtx

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
