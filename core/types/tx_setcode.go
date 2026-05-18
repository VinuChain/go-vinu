// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package types

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

// DelegationPrefix is used by account code to denote delegation to another
// account according to EIP-7702.
var DelegationPrefix = []byte{0xef, 0x01, 0x00}

var ErrAuthorizationValueOverflow = errors.New("authorization value exceeds 256 bits")

// ParseDelegation tries to parse the address from a delegation designator.
func ParseDelegation(b []byte) (common.Address, bool) {
	if len(b) != len(DelegationPrefix)+common.AddressLength || !bytes.HasPrefix(b, DelegationPrefix) {
		return common.Address{}, false
	}
	return common.BytesToAddress(b[len(DelegationPrefix):]), true
}

// AddressToDelegation returns the EIP-7702 delegation designator for addr.
func AddressToDelegation(addr common.Address) []byte {
	return append(append([]byte{}, DelegationPrefix...), addr.Bytes()...)
}

// SetCodeTx implements the EIP-7702 set-code transaction type.
type SetCodeTx struct {
	ChainID    *big.Int
	Nonce      uint64
	GasTipCap  *big.Int
	GasFeeCap  *big.Int
	Gas        uint64
	To         common.Address
	Value      *big.Int
	Data       []byte
	AccessList AccessList
	AuthList   []SetCodeAuthorization

	// Signature values
	V *big.Int `json:"v" gencodec:"required"`
	R *big.Int `json:"r" gencodec:"required"`
	S *big.Int `json:"s" gencodec:"required"`
}

// SetCodeAuthorization is an authorization from an account to delegate its code.
type SetCodeAuthorization struct {
	ChainID *big.Int       `json:"chainId" gencodec:"required"`
	Address common.Address `json:"address" gencodec:"required"`
	Nonce   uint64         `json:"nonce" gencodec:"required"`
	V       uint8          `json:"yParity" gencodec:"required"`
	R       *big.Int       `json:"r" gencodec:"required"`
	S       *big.Int       `json:"s" gencodec:"required"`
}

type setCodeAuthorizationJSON struct {
	ChainID *hexutil.Big    `json:"chainId"`
	Address *common.Address `json:"address"`
	Nonce   *hexutil.Uint64 `json:"nonce"`
	V       *hexutil.Uint64 `json:"yParity"`
	R       *hexutil.Big    `json:"r"`
	S       *hexutil.Big    `json:"s"`
}

// MarshalJSON marshals an EIP-7702 authorization using Ethereum hex quantities.
func (a SetCodeAuthorization) MarshalJSON() ([]byte, error) {
	nonce := hexutil.Uint64(a.Nonce)
	v := hexutil.Uint64(a.V)
	return json.Marshal(&setCodeAuthorizationJSON{
		ChainID: (*hexutil.Big)(a.chainID()),
		Address: &a.Address,
		Nonce:   &nonce,
		V:       &v,
		R:       (*hexutil.Big)(a.R),
		S:       (*hexutil.Big)(a.S),
	})
}

// UnmarshalJSON unmarshals an EIP-7702 authorization.
func (a *SetCodeAuthorization) UnmarshalJSON(input []byte) error {
	var dec setCodeAuthorizationJSON
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.ChainID == nil {
		return errors.New("missing required field 'chainId' in authorization")
	}
	if dec.Address == nil {
		return errors.New("missing required field 'address' in authorization")
	}
	if dec.Nonce == nil {
		return errors.New("missing required field 'nonce' in authorization")
	}
	if dec.V == nil {
		return errors.New("missing required field 'yParity' in authorization")
	}
	v := uint64(*dec.V)
	if v != 0 && v != 1 {
		return ErrInvalidSig
	}
	if dec.R == nil {
		return errors.New("missing required field 'r' in authorization")
	}
	if dec.S == nil {
		return errors.New("missing required field 's' in authorization")
	}
	*a = SetCodeAuthorization{
		ChainID: (*big.Int)(dec.ChainID),
		Address: *dec.Address,
		Nonce:   uint64(*dec.Nonce),
		V:       uint8(v),
		R:       (*big.Int)(dec.R),
		S:       (*big.Int)(dec.S),
	}
	return nil
}

func (a *SetCodeAuthorization) chainID() *big.Int {
	if a == nil || a.ChainID == nil {
		return new(big.Int)
	}
	return a.ChainID
}

func validAuthorizationScalar(x *big.Int) bool {
	return x == nil || x.Sign() >= 0 && x.BitLen() <= 256
}

// ValidateSetCodeAuthorization verifies that authorization tuple scalar fields
// fit the uint256 bounds required by EIP-7702 transaction encoding.
func ValidateSetCodeAuthorization(auth SetCodeAuthorization) error {
	if !validAuthorizationScalar(auth.ChainID) || !validAuthorizationScalar(auth.R) || !validAuthorizationScalar(auth.S) {
		return ErrAuthorizationValueOverflow
	}
	return nil
}

// ValidateSetCodeAuthorizations validates every authorization tuple in auths.
func ValidateSetCodeAuthorizations(auths []SetCodeAuthorization) error {
	for _, auth := range auths {
		if err := ValidateSetCodeAuthorization(auth); err != nil {
			return err
		}
	}
	return nil
}

// SignSetCode creates a signed EIP-7702 authorization.
func SignSetCode(prv *ecdsa.PrivateKey, auth SetCodeAuthorization) (SetCodeAuthorization, error) {
	sighash := auth.SigHash()
	sig, err := crypto.Sign(sighash[:], prv)
	if err != nil {
		return SetCodeAuthorization{}, err
	}
	return SetCodeAuthorization{
		ChainID: new(big.Int).Set(auth.chainID()),
		Address: auth.Address,
		Nonce:   auth.Nonce,
		V:       sig[64],
		R:       new(big.Int).SetBytes(sig[:32]),
		S:       new(big.Int).SetBytes(sig[32:64]),
	}, nil
}

// SigHash returns the hash of the authorization for signing.
func (a *SetCodeAuthorization) SigHash() common.Hash {
	return prefixedRlpHash(0x05, []interface{}{
		a.chainID(),
		a.Address,
		a.Nonce,
	})
}

// Authority recovers the authorizing account of an authorization.
func (a *SetCodeAuthorization) Authority() (common.Address, error) {
	if a == nil || a.R == nil || a.S == nil || !crypto.ValidateSignatureValues(a.V, a.R, a.S, true) {
		return common.Address{}, ErrInvalidSig
	}
	sighash := a.SigHash()
	sig := make([]byte, crypto.SignatureLength)
	r, s := a.R.Bytes(), a.S.Bytes()
	copy(sig[32-len(r):32], r)
	copy(sig[64-len(s):64], s)
	sig[64] = a.V
	pub, err := crypto.Ecrecover(sighash[:], sig)
	if err != nil {
		return common.Address{}, err
	}
	if len(pub) == 0 || pub[0] != 4 {
		return common.Address{}, errors.New("invalid public key")
	}
	var addr common.Address
	copy(addr[:], crypto.Keccak256(pub[1:])[12:])
	return addr, nil
}

// copy creates a deep copy of the transaction data and initializes all fields.
func (tx *SetCodeTx) copy() TxData {
	cpy := &SetCodeTx{
		Nonce: tx.Nonce,
		To:    tx.To,
		Data:  common.CopyBytes(tx.Data),
		Gas:   tx.Gas,
		// These are copied below.
		AccessList: make(AccessList, len(tx.AccessList)),
		AuthList:   make([]SetCodeAuthorization, len(tx.AuthList)),
		Value:      new(big.Int),
		ChainID:    new(big.Int),
		GasTipCap:  new(big.Int),
		GasFeeCap:  new(big.Int),
		V:          new(big.Int),
		R:          new(big.Int),
		S:          new(big.Int),
	}
	copy(cpy.AccessList, tx.AccessList)
	for i := range tx.AuthList {
		cpy.AuthList[i] = tx.AuthList[i]
		if tx.AuthList[i].ChainID != nil {
			cpy.AuthList[i].ChainID = new(big.Int).Set(tx.AuthList[i].ChainID)
		}
		if tx.AuthList[i].R != nil {
			cpy.AuthList[i].R = new(big.Int).Set(tx.AuthList[i].R)
		}
		if tx.AuthList[i].S != nil {
			cpy.AuthList[i].S = new(big.Int).Set(tx.AuthList[i].S)
		}
	}
	if tx.Value != nil {
		cpy.Value.Set(tx.Value)
	}
	if tx.ChainID != nil {
		cpy.ChainID.Set(tx.ChainID)
	}
	if tx.GasTipCap != nil {
		cpy.GasTipCap.Set(tx.GasTipCap)
	}
	if tx.GasFeeCap != nil {
		cpy.GasFeeCap.Set(tx.GasFeeCap)
	}
	if tx.V != nil {
		cpy.V.Set(tx.V)
	}
	if tx.R != nil {
		cpy.R.Set(tx.R)
	}
	if tx.S != nil {
		cpy.S.Set(tx.S)
	}
	return cpy
}

// accessors for innerTx.
func (tx *SetCodeTx) txType() byte           { return SetCodeTxType }
func (tx *SetCodeTx) chainID() *big.Int      { return tx.ChainID }
func (tx *SetCodeTx) accessList() AccessList { return tx.AccessList }
func (tx *SetCodeTx) data() []byte           { return tx.Data }
func (tx *SetCodeTx) gas() uint64            { return tx.Gas }
func (tx *SetCodeTx) gasFeeCap() *big.Int    { return tx.GasFeeCap }
func (tx *SetCodeTx) gasTipCap() *big.Int    { return tx.GasTipCap }
func (tx *SetCodeTx) gasPrice() *big.Int     { return tx.GasFeeCap }
func (tx *SetCodeTx) value() *big.Int        { return tx.Value }
func (tx *SetCodeTx) nonce() uint64          { return tx.Nonce }
func (tx *SetCodeTx) to() *common.Address    { tmp := tx.To; return &tmp }

func (tx *SetCodeTx) rawSignatureValues() (v, r, s *big.Int) {
	return tx.V, tx.R, tx.S
}

func (tx *SetCodeTx) setSignatureValues(chainID, v, r, s *big.Int) {
	tx.ChainID, tx.V, tx.R, tx.S = chainID, v, r, s
}
