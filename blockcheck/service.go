package blockcheck

import (
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/metachris/go-ethutils/addresslookup"
)

type BlockCheckService struct {
	Client         *ethclient.Client
	AddressService *addresslookup.AddressLookupService
}

func NewBlockCheckService(client *ethclient.Client) *BlockCheckService {
	return &BlockCheckService{
		Client:         client,
		AddressService: addresslookup.NewAddressLookupService(client),
	}
}

// func (b *BlockCheckService) GetBlockByNumber(number int64) (*BlockCheck, error) {
// 	block, err := blockswithtx.GetBlockWithTxReceipts(b.Client, number)
// 	if err != nil {
// 		return err
// 	}
// 	return b.CheckBlock(block)

// }

// func (b *BlockCheckService) CheckBlock(block *blockswithtx.BlockWithTxReceipts) error {
// 	// Check for failed TX
// 	// Check for bundles
// }
