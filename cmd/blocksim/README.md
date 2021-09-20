Get miner rewards for a full block (via number or hash) by simulating it with eth_callBundle at an mev-geth instance.
Note: The result already excludes the 2 ETH block reward and any burnt gas fees, it's the actual miner earnings from the transactions.

Example arguments:

    $ go run cmd/blocksim/main.go -mevgeth http://xxx.xxx.xxx.xxx:8545 -number 13100622
    $ go run cmd/blocksim/main.go -mevgeth http://xxx.xxx.xxx.xxx:8545 -hash 0x662f81506bd1d1f7cbefa308261ba94ee63438998cdf085c95081448aaf4cc81

Example output:

    Connected to http://xxx.xxx.xxx.xxx:8545
    Block 13100622 0x662f81506bd1d1f7cbefa308261ba94ee63438998cdf085c95081448aaf4cc81        2021-08-26 11:14:55 +0000 UTC   tx=99           gas=13854382    uncles=0
    Simulation result:
    - CoinbaseDiff:           67391709273784431     0.0674 ETH
    - GasFees:                67391709273784431     0.0674 ETH
    - EthSentToCoinbase:                      0     0.0000 ETH

Transactions:
   1 0x5b9f8480250b56e6e1a954c2db75551c104751133f48540c76afb9f290d34b79 cbD=2.5441, gasFee=2.5441, ethSentToCb=0.0000
   2 0x50e408ea25ed3b0fd8c016bef289e18a8f5d308e1377adfbb3cd88aa5313e30f cbD=2.0123, gasFee=2.0123, ethSentToCb=0.0000
   3 0x6ad722ca388de995b87f1a18da8f66afd9b163bcf5f8e584c1e0f1462b22b220 cbD=1.4072, gasFee=1.4072, ethSentToCb=0.0000
   ...
