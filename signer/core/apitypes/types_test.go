package apitypes

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestSendTxArgsSetCodeRequiresTo(t *testing.T) {
	args := &SendTxArgs{
		ChainID:              (*hexutil.Big)(big.NewInt(206)),
		Gas:                  hexutil.Uint64(50_000),
		MaxFeePerGas:         (*hexutil.Big)(big.NewInt(1)),
		MaxPriorityFeePerGas: (*hexutil.Big)(big.NewInt(1)),
		AuthorizationList: []types.SetCodeAuthorization{{
			ChainID: big.NewInt(206),
			Address: common.HexToAddress("0x1111000000000000000000000000000000000000"),
			R:       big.NewInt(1),
			S:       big.NewInt(1),
		}},
	}
	if _, err := args.ToTransaction(); err == nil {
		t.Fatal("ToTransaction accepted set-code args without to")
	}
}
