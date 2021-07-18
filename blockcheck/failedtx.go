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
