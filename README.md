# Utilities for [Flashbots](https://github.com/flashbots/pm)

* Go API client for the [mev-blocks API](https://blocks.flashbots.net/) for information about Flashbots blocks and transactions
* Detect bundle order errors
* Detect failed Flashbots and other 0-gas transactions (can run over history or in 'watch' mode, webserver that serves recent detections)
* Various Go utilities

Uses:

* https://github.com/metachris/go-ethutils
* https://github.com/ethereum/go-ethereum

Notes:

* You should use IPC connections to the geth node, as there are a lot of API calls (one for each tx).
* PRs and contributions are welcome :)

---

## Flashbots Blocks & Transactions API

https://blocks.flashbots.net/

Installation:

```bash
go get github.com/metachris/flashbots/api
```

Usage:

```go
// Blocks API: default
block, err := api.GetBlocks(nil)

// Blocks API: options
opts := api.GetBlocksOptions{BlockNumber: 12527162}
block, err := api.GetBlocks(&opts)


// Transactions API: default
txs, err := GetTransactions(nil)
```

