package core

import (
	"testing"

	"github.com/ethereum/go-ethereum/params"
)

func TestIntrinsicGasEIP3860AddsInitcodeWordCost(t *testing.T) {
	data := make([]byte, 33)

	preShanghai, err := IntrinsicGas(data, nil, nil, true, true, true, false)
	if err != nil {
		t.Fatal(err)
	}
	shanghai, err := IntrinsicGas(data, nil, nil, true, true, true, true)
	if err != nil {
		t.Fatal(err)
	}

	want := preShanghai + 2*params.InitCodeWordGas
	if shanghai != want {
		t.Fatalf("unexpected Shanghai intrinsic gas: have %d, want %d", shanghai, want)
	}
}
