package runtime

import (
	"bytes"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
)

func cancunTestConfigs() (*params.ChainConfig, *params.ChainConfig) {
	shanghai := *params.TestChainConfig
	shanghai.ShanghaiBlock = big.NewInt(0)
	shanghai.CancunBlock = nil

	cancun := shanghai
	cancun.CancunBlock = big.NewInt(0)
	return &shanghai, &cancun
}

func TestCancunTransientStorageActivation(t *testing.T) {
	shanghai, cancun := cancunTestConfigs()
	code := []byte{
		byte(vm.PUSH1), 0x2a,
		byte(vm.PUSH1), 0x01,
		byte(vm.TSTORE),
		byte(vm.PUSH1), 0x01,
		byte(vm.TLOAD),
		byte(vm.PUSH0),
		byte(vm.MSTORE),
		byte(vm.PUSH1), 0x20,
		byte(vm.PUSH0),
		byte(vm.RETURN),
	}

	_, _, err := Execute(code, nil, &Config{ChainConfig: shanghai})
	var invalid *vm.ErrInvalidOpCode
	if !errors.As(err, &invalid) {
		t.Fatalf("pre-Cancun TSTORE/TLOAD error = %v, want invalid opcode", err)
	}

	ret, _, err := Execute(code, nil, &Config{ChainConfig: cancun})
	if err != nil {
		t.Fatal(err)
	}
	if len(ret) != 32 || ret[31] != 0x2a {
		t.Fatalf("TLOAD return = %x, want 32-byte word ending in 2a", ret)
	}
}

func TestCancunMCOPYActivation(t *testing.T) {
	shanghai, cancun := cancunTestConfigs()
	value := []byte{
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
		0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
		0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f,
	}
	code := []byte{byte(vm.PUSH32)}
	code = append(code, value...)
	code = append(code,
		byte(vm.PUSH1), 0x20,
		byte(vm.MSTORE),
		byte(vm.PUSH1), 0x20,
		byte(vm.PUSH1), 0x20,
		byte(vm.PUSH0),
		byte(vm.MCOPY),
		byte(vm.PUSH1), 0x20,
		byte(vm.PUSH0),
		byte(vm.RETURN),
	)

	_, _, err := Execute(code, nil, &Config{ChainConfig: shanghai})
	var invalid *vm.ErrInvalidOpCode
	if !errors.As(err, &invalid) {
		t.Fatalf("pre-Cancun MCOPY error = %v, want invalid opcode", err)
	}

	ret, _, err := Execute(code, nil, &Config{ChainConfig: cancun})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ret, value) {
		t.Fatalf("MCOPY return = %x, want %x", ret, value)
	}
}
