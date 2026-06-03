package ethapi

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
)

type estimateGasHeaderBackend struct {
	Backend
	config      *params.ChainConfig
	headerCalls int
}

func (b *estimateGasHeaderBackend) ChainConfig() *params.ChainConfig {
	return b.config
}

func (b *estimateGasHeaderBackend) HeaderByNumberOrHash(context.Context, rpc.BlockNumberOrHash) (*types.Header, error) {
	b.headerCalls++
	return nil, nil
}

func TestVinuLatestEVMEstimateGasCap(t *testing.T) {
	cfg := &params.ChainConfig{VinuLatestEVMBlock: big.NewInt(10)}

	if got := vinuLatestEVMEstimateGasCap(cfg, big.NewInt(9), params.MaxTxGasLimit+1); got != params.MaxTxGasLimit+1 {
		t.Fatalf("pre-fork cap = %d, want unchanged", got)
	}
	if got := vinuLatestEVMEstimateGasCap(cfg, big.NewInt(10), params.MaxTxGasLimit+1); got != params.MaxTxGasLimit {
		t.Fatalf("post-fork cap = %d, want %d", got, params.MaxTxGasLimit)
	}
	if got := vinuLatestEVMEstimateGasCap(cfg, big.NewInt(10), params.TxGas); got != params.TxGas {
		t.Fatalf("below-cap gas = %d, want unchanged", got)
	}
}

func TestVinuLatestEVMEstimateGasMayNeedCap(t *testing.T) {
	cfg := &params.ChainConfig{VinuLatestEVMBlock: big.NewInt(10)}
	if !vinuLatestEVMEstimateGasMayNeedCap(cfg, params.MaxTxGasLimit+1) {
		t.Fatal("configured fork with over-cap gas should require block lookup")
	}
	if vinuLatestEVMEstimateGasMayNeedCap(cfg, params.MaxTxGasLimit) {
		t.Fatal("at-cap gas should not require block lookup")
	}
	if vinuLatestEVMEstimateGasMayNeedCap(&params.ChainConfig{}, params.MaxTxGasLimit+1) {
		t.Fatal("unconfigured fork should not require block lookup")
	}
}

func TestDoEstimateGasExplicitOverCapChecksConfiguredVinuForkHeader(t *testing.T) {
	gas := hexutil.Uint64(params.MaxTxGasLimit + 1)
	backend := &estimateGasHeaderBackend{
		config: &params.ChainConfig{VinuLatestEVMBlock: big.NewInt(10)},
	}

	_, err := DoEstimateGas(
		context.Background(),
		backend,
		TransactionArgs{Gas: &gas},
		rpc.BlockNumberOrHashWithNumber(rpc.LatestBlockNumber),
		0,
	)
	if err == nil || !strings.Contains(err.Error(), "header not found") {
		t.Fatalf("DoEstimateGas error = %v, want header-not-found from cap lookup", err)
	}
	if backend.headerCalls != 1 {
		t.Fatalf("HeaderByNumberOrHash calls = %d, want 1", backend.headerCalls)
	}
}

func TestVinuLatestEVMEstimateGasBlockNumber(t *testing.T) {
	latest := rpc.BlockNumberOrHashWithNumber(rpc.LatestBlockNumber)
	if got := vinuLatestEVMEstimateGasBlockNumber(latest, big.NewInt(9)); got.Cmp(big.NewInt(10)) != 0 {
		t.Fatalf("latest estimate block number = %v, want 10", got)
	}

	pending := rpc.BlockNumberOrHashWithNumber(rpc.PendingBlockNumber)
	if got := vinuLatestEVMEstimateGasBlockNumber(pending, big.NewInt(10)); got.Cmp(big.NewInt(10)) != 0 {
		t.Fatalf("pending estimate block number = %v, want header number unchanged", got)
	}

	exact := rpc.BlockNumberOrHashWithNumber(rpc.BlockNumber(9))
	if got := vinuLatestEVMEstimateGasBlockNumber(exact, big.NewInt(9)); got.Cmp(big.NewInt(9)) != 0 {
		t.Fatalf("exact estimate block number = %v, want unchanged", got)
	}
}
