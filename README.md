# Utilities for [Flashbots](https://github.com/flashbots/pm)

* Go API client for the [mev-blocks API](https://blocks.flashbots.net/) for information about Flashbots blocks and transactions
* Detect bundle errors: (a) out of order, (b) lower gas fee than lowest non-fb tx
* Detect failed Flashbots and other 0-gas transactions (can run over history or in 'watch' mode, webserver that serves recent detections)
* Various related utilities

Uses:

* https://github.com/ethereum/go-ethereum
* https://github.com/metachris/go-ethutils

Notes:

* There are a lot of API calls (one for each tx), it will only be fast if you are on (or close to) the geth node.
* Ideas, feedback and contributions are welcome.

Reach out: [twitter.com/metachris](https://twitter.com/metachris)

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

---

## Getting Started

Good starting points:
* `cmd/api-test/main.go`
* `cmd/block-watch/main.go`
