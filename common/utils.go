package common

import (
	"math/big"
)

func BigFloatToEString(f *big.Float, prec int) string {
	s1 := f.Text('f', 0)
	if len(s1) >= 16 {
		f2 := new(big.Float).Quo(f, big.NewFloat(1e18))
		s := f2.Text('f', prec)
		return s + "e+18"
	} else if len(s1) >= 9 {
		f2 := new(big.Float).Quo(f, big.NewFloat(1e9))
		s := f2.Text('f', prec)
		return s + "e+09"
	}
	return f.Text('f', prec)
}

func BigIntToEString(i *big.Int, prec int) string {
	f := new(big.Float)
	f.SetInt(i)
	s1 := f.Text('f', 0)
	if len(s1) < 9 {
		return i.String()
	}
	return BigFloatToEString(f, prec)
}
