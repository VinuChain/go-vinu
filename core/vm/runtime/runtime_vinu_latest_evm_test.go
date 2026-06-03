package runtime

import (
	"bytes"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
)

func vinuLatestEVMTestConfigs() (*params.ChainConfig, *params.ChainConfig) {
	cancun := *params.TestChainConfig
	cancun.ShanghaiBlock = big.NewInt(0)
	cancun.CancunBlock = big.NewInt(0)
	cancun.PragueBlock = big.NewInt(0)
	cancun.VinuBLSBlock = big.NewInt(0)
	cancun.VinuLatestEVMBlock = nil

	latest := cancun
	latest.VinuLatestEVMBlock = big.NewInt(0)
	return &cancun, &latest
}

func TestVinuLatestEVMCLZActivation(t *testing.T) {
	cancun, latest := vinuLatestEVMTestConfigs()
	code := []byte{
		byte(vm.PUSH0),
		byte(vm.CLZ),
		byte(vm.PUSH0),
		byte(vm.MSTORE),
		byte(vm.PUSH1), 0x20,
		byte(vm.PUSH0),
		byte(vm.RETURN),
	}

	_, _, err := Execute(code, nil, &Config{ChainConfig: cancun})
	var invalid *vm.ErrInvalidOpCode
	if !errors.As(err, &invalid) {
		t.Fatalf("pre-VinuLatestEVM CLZ error = %v, want invalid opcode", err)
	}

	ret, _, err := Execute(code, nil, &Config{ChainConfig: latest})
	if err != nil {
		t.Fatal(err)
	}
	want := common.LeftPadBytes([]byte{0x01, 0x00}, 32)
	if !bytes.Equal(ret, want) {
		t.Fatalf("CLZ(0) return = %x, want %x", ret, want)
	}
}

func TestVinuLatestEVMCLZValues(t *testing.T) {
	_, latest := vinuLatestEVMTestConfigs()
	for _, tt := range []struct {
		name  string
		value []byte
		want  byte
	}{
		{name: "high bit", value: common.FromHex("0x8000000000000000000000000000000000000000000000000000000000000000"), want: 0},
		{name: "second bit", value: common.FromHex("0x4000000000000000000000000000000000000000000000000000000000000000"), want: 1},
		{name: "one", value: common.LeftPadBytes([]byte{1}, 32), want: 255},
	} {
		t.Run(tt.name, func(t *testing.T) {
			code := append([]byte{byte(vm.PUSH32)}, tt.value...)
			code = append(code,
				byte(vm.CLZ),
				byte(vm.PUSH0),
				byte(vm.MSTORE),
				byte(vm.PUSH1), 0x20,
				byte(vm.PUSH0),
				byte(vm.RETURN),
			)
			ret, _, err := Execute(code, nil, &Config{ChainConfig: latest})
			if err != nil {
				t.Fatal(err)
			}
			want := common.LeftPadBytes([]byte{tt.want}, 32)
			if !bytes.Equal(ret, want) {
				t.Fatalf("CLZ return = %x, want %x", ret, want)
			}
		})
	}
}
