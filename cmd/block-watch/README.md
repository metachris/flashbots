Example blocks:

```bash
# Too low gas fee:
go run cmd/block-watch/*.go -block 12693354 

# Bundle out of order:
go run cmd/block-watch/*.go -block 12699873 

# Failed Flashbots tx:
go run cmd/block-watch/*.go -block 12705543 

# Failed 0-gas-and-data (non-fb) tx:
go run cmd/block-watch/*.go -block 12605331
```


## TODO

* ErrorCount struct method to add counts of another ErrorCount struct to self
* discord.go should just accept a blockcheck struct and create the right message there
