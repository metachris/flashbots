package flashbotsfailedtx

type FailedTx struct {
	Hash        string
	From        string
	To          string
	Block       uint64
	IsFlashbots bool // if false then it's a failed 0-gas tx but not from Flashbots
}
