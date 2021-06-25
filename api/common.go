// Client for Flashbots mev-blocks API: https://blocks.flashbots.net/
package api

type FlashbotsBlock struct {
	BlockNumber       int64  `json:"block_number"`
	Miner             string `json:"miner"`
	MinerReward       string `json:"miner_reward"`
	CoinbaseTransfers string `json:"coinbase_transfers"`

	GasUsed      int64                  `json:"gas_used"`
	GasPrice     string                 `json:"gas_price"`
	Transactions []FlashbotsTransaction `json:"transactions"`
}

type FlashbotsTransaction struct {
	Hash             string `json:"transaction_hash"`
	TxIndex          int64  `json:"tx_index"`
	BundleIndex      int64  `json:"bundle_index"`
	BlockNumber      int64  `json:"block_number"`
	EoaAddress       string `json:"eoa_address"`
	ToAddress        string `json:"to_address"`
	GasUsed          int64  `json:"gas_used"`
	GasPrice         string `json:"gas_price"`
	CoinbaseTransfer string `json:"coinbase_transfer"`
	TotalMinerReward string `json:"total_miner_reward"`
}

// HasTx returns true if the transaction hash is included in any of the blocks of the API response
func (b *FlashbotsBlock) HasTx(hash string) bool {
	for _, tx := range b.Transactions {
		if tx.Hash == hash {
			return true
		}
	}
	return false
}
