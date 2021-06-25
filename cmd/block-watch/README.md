Example blocks:

```bash
# Too low gas fee:
go run cmd/block-watch/*.go -block 12693354 

# Bundle out of order:
go run cmd/block-watch/*.go -block 12693354 

# Failed Flashbots tx:
go run cmd/block-watch/*.go -block 12693354 

# Failed 0-gas tx:
go run cmd/block-watch/*.go -block 12693354 
```
