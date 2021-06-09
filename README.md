# Flashbots failed transactions

Iterate over Ethereum blocks to get failed [Flashbots](https://github.com/flashbots/pm) transactions.

Uses https://github.com/metachris/go-ethutils

Notes:

* You can set the geth node URI as environment variable `ETH_NODE`, or pass it in as `-eth` argument.
* You should use an IPC connection to the geth node, as there are a lot of API calls (one for each tx).

```bash
# Subscribe to new blocks and find failed Flashbots tx:
go run cmd/flashbots-failed-tx/main.go -watch
go run cmd/flashbots-failed-tx/main.go -watch -silent  # print only failed transactions

# Historic, using a starting block
go run cmd/flashbots-failed-tx/main.go -block 12539827           # 1 block
go run cmd/flashbots-failed-tx/main.go -block 12590513
go run cmd/flashbots-failed-tx/main.go -block 12539827 -len 5    # 5 blocks
go run cmd/flashbots-failed-tx/main.go -block 12539827 -len 10m  # all blocks within 10 minutes of given block
go run cmd/flashbots-failed-tx/main.go -block 12539827 -len 1h   # all blocks within 1 hour of given block
go run cmd/flashbots-failed-tx/main.go -block 12539827 -len 1d   # all blocks within 1 day of given block
go run cmd/flashbots-failed-tx/main.go -block 12539827 -len 1d -silent  # don't print information for every block

# Historic, using a starting date
go run cmd/flashbots-failed-tx/main.go -date -1d -len 1h         # all blocks within 1 hour of yesterday 00:00:00 (UTC)
go run cmd/flashbots-failed-tx/main.go -date 2021-05-31 -len 1d  # all blocks from this day (00:00:00 -> 23:59:59 UTC)
go run cmd/flashbots-failed-tx/main.go -date 2021-05-31 -hour 3 -min 53 -len 5m  # all blocks within 1 hour of given date and time (UTC)
```

You can also install this tool as `flashbots-failed-tx` binary:

```bash
go install github.com/metachris/flashbots-failed-tx/cmd/flashbots-failed-tx@master
flashbots-failed-tx -h
```


![Screenshot](https://user-images.githubusercontent.com/116939/120549797-532fa500-c3f4-11eb-84fc-1e02d1db4cd6.png)


Interesting blocks:

* `12539827` - 1 failed 0-gas tx
* `12590513` - ~100 failed 0-gas tx
* `12527162` - failed Flashbots tx
