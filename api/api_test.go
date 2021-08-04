// Client for [Flashbots mev-blocks API](https://blocks.flashbots.net/)
package api_test

import (
	"testing"

	"github.com/metachris/flashbots/api"
)

func TestBlocksApi(t *testing.T) {
	opts := api.GetBlocksOptions{}
	if opts.ToUriQuery() != "" {
		t.Error("Should be empty, is", opts.ToUriQuery())
	}

	opts = api.GetBlocksOptions{BlockNumber: 123}
	if opts.ToUriQuery() != "?block_number=123" {
		t.Error("Wrong ToUriQuery:", opts, opts.ToUriQuery())
	}

	opts = api.GetBlocksOptions{BlockNumber: 123, Miner: "xxx"}
	if opts.ToUriQuery() != "?block_number=123&miner=xxx" {
		t.Error("Wrong ToUriQuery:", opts, opts.ToUriQuery())
	}

	opts = api.GetBlocksOptions{BlockNumber: 12527162}
	block, err := api.GetBlocks(&opts)
	if err != nil {
		t.Error(err)
	}

	if len(block.Blocks) != 1 {
		t.Error("Wrong amount of blocks:", len(block.Blocks))
	}

	if len(block.GetTxMap()) != 18 {
		t.Error("Wrong amount of tx:", len(block.GetTxMap()))
	}

	tx1 := "0x50aa84a35a999f7dbfed2d72c44712742edbfa12dfdeb33904e3fe7244791eed"
	if !block.HasTx(tx1) {
		t.Error("Should be a failed Flashbots tx", tx1)
	}
}

func TestTransactionsApi(t *testing.T) {
	opts := api.GetTransactionsOptions{}
	if opts.ToUriQuery() != "" {
		t.Error("Should be empty, is", opts.ToUriQuery())
	}

	// opts = GetTransactionsOptions{}
	txs, err := api.GetTransactions(nil)
	if err != nil {
		t.Error(err)
	}

	if len(txs.Transactions) != 100 {
		t.Error("Wrong amount of tx:", len(txs.Transactions))
	}

	txs, err = api.GetTransactions(&api.GetTransactionsOptions{Limit: 5})
	if err != nil {
		t.Error(err)
	}

	if len(txs.Transactions) != 5 {
		t.Error("Wrong amount of tx:", len(txs.Transactions), "wanted:", 5)
	}
}
