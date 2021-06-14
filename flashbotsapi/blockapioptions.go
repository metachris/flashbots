package flashbotsapi

import (
	"fmt"
	"strings"
)

type BlockApiOptions struct {
	BlockNumber int64
	Miner       string
	From        string
	Before      int64
	Limit       int64
}

func (b BlockApiOptions) ToUriQuery() string {
	args := []string{}
	if b.BlockNumber > 0 {
		args = append(args, fmt.Sprintf("block_number=%d", b.BlockNumber))
	}
	if b.Miner != "" {
		args = append(args, fmt.Sprintf("miner=%s", b.Miner))
	}
	if b.From != "" {
		args = append(args, fmt.Sprintf("from=%s", b.From))
	}
	if b.Before > 0 {
		args = append(args, fmt.Sprintf("before=%d", b.Before))
	}
	if b.Limit > 0 {
		args = append(args, fmt.Sprintf("limit=%d", b.Limit))
	}

	s := strings.Join(args, "&")
	if len(s) > 0 {
		s = "?" + s
	}

	return s
}
