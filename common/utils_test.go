package common

import (
	"math/big"
	"testing"
)

func TestBigFloatToEString(t *testing.T) {
	f19 := big.NewFloat(2255212009613187301)
	f18 := big.NewFloat(225521200961318730)
	f17 := big.NewFloat(22552120096131873)
	f16 := big.NewFloat(2255212009613187)
	f10 := big.NewFloat(2255212009610)
	f9 := big.NewFloat(225521200961)
	f8 := big.NewFloat(225521200961)
	f3 := big.NewFloat(225)

	s := BigFloatToEString(f19, 3)
	if s != "2.255e+18" {
		t.Error("Unexpected result string from BigFloatToEString:", s)
	}

	s = BigFloatToEString(f18, 3)
	if s != "0.226e+18" {
		t.Error("Unexpected result string from BigFloatToEString:", s)
	}

	s = BigFloatToEString(f17, 3)
	if s != "0.023e+18" {
		t.Error("Unexpected result string from BigFloatToEString:", s)
	}

	s = BigFloatToEString(f16, 3)
	if s != "2255212.010e+09" {
		t.Error("Unexpected result string from BigFloatToEString:", s)
	}

	s = BigFloatToEString(f10, 3)
	if s != "2255.212e+09" {
		t.Error("Unexpected result string from BigFloatToEString:", s)
	}

	s = BigFloatToEString(f9, 3)
	if s != "225.521e+09" {
		t.Error("Unexpected result string from BigFloatToEString:", s)
	}

	s = BigFloatToEString(f8, 3)
	if s != "225.521e+09" {
		t.Error("Unexpected result string from BigFloatToEString:", s)
	}

	s = BigFloatToEString(f3, 3)
	if s != "225.000" {
		t.Error("Unexpected result string from BigFloatToEString:", s)
	}
}

func TestBigIntToEString(t *testing.T) {
	i19 := big.NewInt(2255212009613187301)
	s := BigIntToEString(i19, 3)
	if s != "2.255e+18" {
		t.Error("Unexpected result string from BigFloatToEString:", s)
	}
}
