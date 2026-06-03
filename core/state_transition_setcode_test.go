package core

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
)

func TestIntrinsicGasSetCodeAuthorizationCost(t *testing.T) {
	auths := []types.SetCodeAuthorization{
		{ChainID: big.NewInt(206), Address: common.Address{}, Nonce: 1, R: new(big.Int), S: new(big.Int)},
		{ChainID: big.NewInt(206), Address: common.Address{}, Nonce: 2, R: new(big.Int), S: new(big.Int)},
	}
	gas, err := IntrinsicGas(nil, nil, auths, false, true, true, true)
	if err != nil {
		t.Fatal(err)
	}
	want := params.TxGas + uint64(len(auths))*params.CallNewAccountGas
	if gas != want {
		t.Fatalf("intrinsic gas = %d, want %d", gas, want)
	}
}

func setCodeTestChainConfig(prague bool) *params.ChainConfig {
	cfg := *params.TestChainConfig
	cfg.HomesteadBlock = common.Big0
	cfg.IstanbulBlock = common.Big0
	cfg.BerlinBlock = common.Big0
	cfg.LondonBlock = common.Big0
	cfg.ShanghaiBlock = common.Big0
	cfg.CancunBlock = common.Big0
	if prague {
		cfg.PragueBlock = common.Big0
	} else {
		cfg.PragueBlock = nil
	}
	return &cfg
}

func vinuLatestEVMStateTestChainConfig(active bool) *params.ChainConfig {
	cfg := *setCodeTestChainConfig(true)
	cfg.VinuBLSBlock = common.Big0
	if active {
		cfg.VinuLatestEVMBlock = common.Big0
	} else {
		cfg.VinuLatestEVMBlock = nil
	}
	return &cfg
}

func TestTransitionDbRejectsTransactionAboveVinuLatestEVMGasCap(t *testing.T) {
	sender := common.HexToAddress("0x1111")
	receiver := common.HexToAddress("0x2222")
	gasLimit := params.MaxTxGasLimit + 1

	for _, tt := range []struct {
		name      string
		active    bool
		wantError error
	}{
		{name: "pre-vinu-latest-evm", active: false},
		{name: "vinu-latest-evm", active: true, wantError: ErrTxGasLimitExceeded},
	} {
		t.Run(tt.name, func(t *testing.T) {
			statedb, err := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
			if err != nil {
				t.Fatal(err)
			}
			statedb.SetBalance(sender, new(big.Int).SetUint64(gasLimit+params.TxGas))
			evm := vm.NewEVM(vm.BlockContext{
				CanTransfer: CanTransfer,
				Transfer:    Transfer,
				BlockNumber: big.NewInt(1),
				BaseFee:     big.NewInt(0),
				GasLimit:    gasLimit,
			}, vm.TxContext{}, statedb, vinuLatestEVMStateTestChainConfig(tt.active), vm.Config{})
			msg := types.NewMessage(
				sender,
				&receiver,
				0,
				big.NewInt(0),
				gasLimit,
				big.NewInt(1),
				big.NewInt(1),
				big.NewInt(1),
				nil,
				nil,
				false,
			)

			_, err = ApplyMessage(evm, msg, new(GasPool).AddGas(gasLimit))
			if !errors.Is(err, tt.wantError) {
				t.Fatalf("ApplyMessage error = %v, want %v", err, tt.wantError)
			}
		})
	}
}

func TestTransitionDbGatesDelegatedSenderByPrague(t *testing.T) {
	const gasLimit = uint64(50_000)
	sender := common.HexToAddress("0x1111")
	receiver := common.HexToAddress("0x2222")
	target := common.HexToAddress("0x3333")

	for _, tt := range []struct {
		name      string
		prague    bool
		wantError error
	}{
		{name: "pre-prague", prague: false, wantError: ErrSenderNoEOA},
		{name: "prague", prague: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			statedb, err := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
			if err != nil {
				t.Fatal(err)
			}
			statedb.SetBalance(sender, big.NewInt(1_000_000_000))
			statedb.SetCode(sender, types.AddressToDelegation(target))

			evm := vm.NewEVM(vm.BlockContext{
				CanTransfer: CanTransfer,
				Transfer:    Transfer,
				BlockNumber: big.NewInt(1),
				BaseFee:     big.NewInt(0),
				GasLimit:    gasLimit,
			}, vm.TxContext{}, statedb, setCodeTestChainConfig(tt.prague), vm.Config{})
			msg := types.NewMessage(
				sender,
				&receiver,
				0,
				big.NewInt(0),
				gasLimit,
				big.NewInt(1),
				big.NewInt(1),
				big.NewInt(1),
				nil,
				nil,
				false,
			)

			_, err = ApplyMessage(evm, msg, new(GasPool).AddGas(gasLimit))
			if !errors.Is(err, tt.wantError) {
				t.Fatalf("ApplyMessage error = %v, want %v", err, tt.wantError)
			}
		})
	}
}

func TestTransitionDbRejectsOverUint256AuthorizationScalar(t *testing.T) {
	const gasLimit = uint64(100_000)
	sender := common.HexToAddress("0x1111")
	receiver := common.HexToAddress("0x2222")
	tooBig := new(big.Int).Lsh(big.NewInt(1), 256)

	statedb, err := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	if err != nil {
		t.Fatal(err)
	}
	statedb.SetBalance(sender, big.NewInt(1_000_000_000))

	evm := vm.NewEVM(vm.BlockContext{
		CanTransfer: CanTransfer,
		Transfer:    Transfer,
		BlockNumber: big.NewInt(1),
		BaseFee:     big.NewInt(0),
		GasLimit:    gasLimit,
	}, vm.TxContext{}, statedb, setCodeTestChainConfig(true), vm.Config{})
	msg := types.NewMessageWithSetCodeAuthorizations(
		sender,
		&receiver,
		0,
		big.NewInt(0),
		gasLimit,
		big.NewInt(1),
		big.NewInt(1),
		big.NewInt(1),
		nil,
		nil,
		[]types.SetCodeAuthorization{{
			ChainID: tooBig,
			R:       big.NewInt(1),
			S:       big.NewInt(1),
		}},
		false,
	)
	_, err = ApplyMessage(evm, msg, new(GasPool).AddGas(gasLimit))
	if !errors.Is(err, types.ErrAuthorizationValueOverflow) {
		t.Fatalf("ApplyMessage error = %v, want %v", err, types.ErrAuthorizationValueOverflow)
	}
}
