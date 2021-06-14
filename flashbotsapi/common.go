// Client for Flashbots mev-blocks API: https://blocks.flashbots.net/
package flashbotsapi

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
