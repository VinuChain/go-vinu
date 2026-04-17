package ethapi

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/params"
)

// newTestStateDB creates a minimal in-memory StateDB suitable for unit tests.
func newTestStateDB(t *testing.T) *state.StateDB {
	t.Helper()
	db := rawdb.NewMemoryDatabase()
	sdb, err := state.New(common.Hash{}, state.NewDatabase(db), nil)
	if err != nil {
		t.Fatalf("state.New: %v", err)
	}
	return sdb
}

// TestStateOverrideApply_CodeSizeLimit verifies that Apply rejects code blobs
// larger than params.MaxCodeSize in a state override.
func TestStateOverrideApply_CodeSizeLimit(t *testing.T) {
	oversized := make(hexutil.Bytes, params.MaxCodeSize+1)
	override := StateOverride{
		common.Address{1}: OverrideAccount{Code: &oversized},
	}
	sdb := newTestStateDB(t)
	if err := override.Apply(sdb); err == nil {
		t.Fatalf("Apply must reject code larger than params.MaxCodeSize (%d); got nil error", params.MaxCodeSize)
	}
}

// TestStateOverrideApply_CodeSizeLimit_ExactBoundary verifies that code of exactly
// params.MaxCodeSize is accepted (boundary condition).
func TestStateOverrideApply_CodeSizeLimit_ExactBoundary(t *testing.T) {
	exact := make(hexutil.Bytes, params.MaxCodeSize)
	override := StateOverride{
		common.Address{1}: OverrideAccount{Code: &exact},
	}
	sdb := newTestStateDB(t)
	if err := override.Apply(sdb); err != nil {
		t.Fatalf("Apply must accept code of exactly MaxCodeSize; got err: %v", err)
	}
}

// TestStateOverrideApply_CodeSizeLimit_NilCode verifies that a nil Code field
// is a no-op (no crash or spurious error).
func TestStateOverrideApply_CodeSizeLimit_NilCode(t *testing.T) {
	override := StateOverride{
		common.Address{1}: OverrideAccount{Code: nil},
	}
	sdb := newTestStateDB(t)
	if err := override.Apply(sdb); err != nil {
		t.Fatalf("Apply with nil Code must succeed; got err: %v", err)
	}
}
