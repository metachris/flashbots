package blockcheck

type MinerErrors struct {
	MinerHash string
	MinerName string

	Blocks      map[int64]bool // To avoid counting errors / blocks twice
	ErrorCounts ErrorCounts
}

func NewMinerErrorCounter() MinerErrors {
	return MinerErrors{
		Blocks: make(map[int64]bool),
	}
}

func (ec *MinerErrors) AddErrorCounts(block int64, counts ErrorCounts) {
	ec.ErrorCounts.Add(counts)
	ec.Blocks[block] = true
}
