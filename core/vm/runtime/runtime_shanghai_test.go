package runtime

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
)

func TestShanghaiPUSH0Activation(t *testing.T) {
	code := common.FromHex("0x5f60005260206000f3")
	london := *params.TestChainConfig
	london.ShanghaiBlock = nil
	london.CancunBlock = nil

	_, _, err := Execute(code, nil, &Config{ChainConfig: &london})
	var invalid *vm.ErrInvalidOpCode
	if !errors.As(err, &invalid) {
		t.Fatalf("pre-Shanghai PUSH0 error = %v, want invalid opcode", err)
	}

	shanghai := london
	shanghai.ShanghaiBlock = big.NewInt(0)
	shanghai.CancunBlock = nil
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

func TestShanghaiWarmCoinbaseReducesBalanceGas(t *testing.T) {
	coinbase := common.HexToAddress("0x000000000000000000000000000000000000c0de")
	contract := common.HexToAddress("0x100")
	code := append([]byte{byte(vm.PUSH20)}, coinbase.Bytes()...)
	code = append(code, byte(vm.BALANCE), byte(vm.POP), byte(vm.STOP))

	london := *params.TestChainConfig
	london.ShanghaiBlock = nil
	london.CancunBlock = nil
	shanghai := london
	shanghai.ShanghaiBlock = big.NewInt(0)

	run := func(config *params.ChainConfig) uint64 {
		statedb, err := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
		if err != nil {
			t.Fatal(err)
		}
		statedb.CreateAccount(contract)
		statedb.SetCode(contract, code)
		statedb.Prepare(common.Hash{}, 0)

		_, left, err := Call(contract, nil, &Config{
			ChainConfig: config,
			Coinbase:    coinbase,
			GasLimit:    100000,
			State:       statedb,
		})
		if err != nil {
			t.Fatal(err)
		}
		return left
	}

	londonLeft := run(&london)
	shanghaiLeft := run(&shanghai)
	if shanghaiLeft <= londonLeft {
		t.Fatalf("Shanghai coinbase BALANCE left gas = %d, want greater than pre-Shanghai %d", shanghaiLeft, londonLeft)
	}
}
