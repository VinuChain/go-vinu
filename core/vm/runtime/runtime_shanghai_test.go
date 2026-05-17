package runtime

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
)

func TestShanghaiPUSH0Activation(t *testing.T) {
	code := common.FromHex("0x5f60005260206000f3")
	london := *params.TestChainConfig
	london.ShanghaiBlock = nil

	_, _, err := Execute(code, nil, &Config{ChainConfig: &london})
	var invalid *vm.ErrInvalidOpCode
	if !errors.As(err, &invalid) {
		t.Fatalf("pre-Shanghai PUSH0 error = %v, want invalid opcode", err)
	}

	shanghai := london
	shanghai.ShanghaiBlock = big.NewInt(0)
	ret, _, err := Execute(code, nil, &Config{ChainConfig: &shanghai})
	if err != nil {
		t.Fatal(err)
	}
	if len(ret) != 32 {
		t.Fatalf("PUSH0 return length = %d, want 32", len(ret))
	}
	for i, b := range ret {
		if b != 0 {
			t.Fatalf("PUSH0 return byte %d = %#x, want 0", i, b)
		}
	}
}

func TestShanghaiWarmsCoinbase(t *testing.T) {
	coinbase := common.HexToAddress("0x000000000000000000000000000000000000c0de")
	_, state, err := Execute([]byte{byte(vm.STOP)}, nil, &Config{
		Coinbase: coinbase,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !state.AddressInAccessList(coinbase) {
		t.Fatalf("coinbase %s was not warmed under Shanghai rules", coinbase)
	}
}
