package types

import (
	"bytes"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestParseDelegation(t *testing.T) {
	target := common.HexToAddress("0x1234567890123456789012345678901234567890")
	code := AddressToDelegation(target)
	if !bytes.Equal(code[:len(DelegationPrefix)], DelegationPrefix) {
		t.Fatalf("delegation prefix = %x, want %x", code[:len(DelegationPrefix)], DelegationPrefix)
	}
	got, ok := ParseDelegation(code)
	if !ok {
		t.Fatal("delegation code did not parse")
	}
	if got != target {
		t.Fatalf("delegation target = %s, want %s", got, target)
	}
	if _, ok := ParseDelegation(append(code, 0x00)); ok {
		t.Fatal("oversized delegation code parsed")
	}
	if _, ok := ParseDelegation([]byte{0xef, 0x01, 0x01}); ok {
		t.Fatal("wrong delegation prefix parsed")
	}
}

func TestSetCodeAuthorizationSignRecover(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	target := common.HexToAddress("0x2000000000000000000000000000000000000000")
	auth, err := SignSetCode(key, SetCodeAuthorization{
		ChainID: big.NewInt(206),
		Address: target,
		Nonce:   7,
	})
	if err != nil {
		t.Fatal(err)
	}
	authority, err := auth.Authority()
	if err != nil {
		t.Fatal(err)
	}
	if want := crypto.PubkeyToAddress(key.PublicKey); authority != want {
		t.Fatalf("authority = %s, want %s", authority, want)
	}
}

func TestSetCodeTxPragueSigner(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	to := common.HexToAddress("0x3000000000000000000000000000000000000000")
	authKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	auth, err := SignSetCode(authKey, SetCodeAuthorization{
		ChainID: big.NewInt(206),
		Address: common.HexToAddress("0x4000000000000000000000000000000000000000"),
		Nonce:   0,
	})
	if err != nil {
		t.Fatal(err)
	}
	signer := NewPragueSigner(big.NewInt(206))
	tx, err := SignNewTx(key, signer, &SetCodeTx{
		ChainID:    big.NewInt(206),
		Nonce:      1,
		GasTipCap:  big.NewInt(2),
		GasFeeCap:  big.NewInt(100),
		Gas:        50_000,
		To:         to,
		Value:      new(big.Int),
		AuthList:   []SetCodeAuthorization{auth},
		AccessList: AccessList{},
	})
	if err != nil {
		t.Fatal(err)
	}
	from, err := Sender(signer, tx)
	if err != nil {
		t.Fatal(err)
	}
	if want := crypto.PubkeyToAddress(key.PublicKey); from != want {
		t.Fatalf("sender = %s, want %s", from, want)
	}
	if _, err := Sender(NewLondonSigner(big.NewInt(206)), tx); err != ErrTxTypeNotSupported {
		t.Fatalf("London signer error = %v, want %v", err, ErrTxTypeNotSupported)
	}
}

func TestSetCodeAuthorizationRejectsOverUint256RLP(t *testing.T) {
	tooBig := new(big.Int).Lsh(big.NewInt(1), 256)
	tx := NewTx(&SetCodeTx{
		ChainID:   big.NewInt(206),
		GasTipCap: big.NewInt(1),
		GasFeeCap: big.NewInt(1),
		Gas:       50_000,
		To:        common.HexToAddress("0x3000000000000000000000000000000000000000"),
		Value:     new(big.Int),
		AuthList: []SetCodeAuthorization{
			{
				ChainID: tooBig,
				Address: common.HexToAddress("0x4000000000000000000000000000000000000000"),
				R:       big.NewInt(1),
				S:       big.NewInt(1),
			},
		},
		V: new(big.Int),
		R: new(big.Int),
		S: new(big.Int),
	})
	blob, err := tx.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	var decoded Transaction
	if err := decoded.UnmarshalBinary(blob); !errors.Is(err, ErrAuthorizationValueOverflow) {
		t.Fatalf("UnmarshalBinary error = %v, want %v", err, ErrAuthorizationValueOverflow)
	}
}
