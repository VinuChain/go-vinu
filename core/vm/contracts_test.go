// Copyright 2017 The go-ethereum Authors
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

package vm

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"math/big"
	"testing"
	"time"

	bls12381 "github.com/consensys/gnark-crypto/ecc/bls12-381"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
)

// precompiledTest defines the input/output pairs for precompiled contract tests.
type precompiledTest struct {
	Input, Expected string
	Gas             uint64
	Name            string
	NoBenchmark     bool // Benchmark primarily the worst-cases
}

// precompiledFailureTest defines the input/error pairs for precompiled
// contract failure tests.
type precompiledFailureTest struct {
	Input         string
	ExpectedError string
	Name          string
}

// allPrecompiles does not map to the actual set of precompiles, as it also contains
// repriced versions of precompiles at certain slots
var allPrecompiles = map[common.Address]PrecompiledContract{
	common.BytesToAddress([]byte{1}):    &ecrecover{},
	common.BytesToAddress([]byte{2}):    &sha256hash{},
	common.BytesToAddress([]byte{3}):    &ripemd160hash{},
	common.BytesToAddress([]byte{4}):    &dataCopy{},
	common.BytesToAddress([]byte{5}):    &bigModExp{eip2565: false},
	common.BytesToAddress([]byte{0xf5}): &bigModExp{eip2565: true},
	common.BytesToAddress([]byte{6}):    &bn256AddIstanbul{},
	common.BytesToAddress([]byte{7}):    &bn256ScalarMulIstanbul{},
	common.BytesToAddress([]byte{8}):    &bn256PairingIstanbul{},
	common.BytesToAddress([]byte{9}):    &blake2F{},
	common.HexToAddress("0x0f0a"):       &bls12381G1Add{},
	common.HexToAddress("0x0f0b"):       &bls12381G1MultiExp{},
	common.HexToAddress("0x0f0c"):       &bls12381G2Add{},
	common.HexToAddress("0x0f0d"):       &bls12381G2MultiExp{},
	common.HexToAddress("0x0f0e"):       &bls12381Pairing{},
	common.HexToAddress("0x0f0f"):       &bls12381MapG1{},
	common.HexToAddress("0x0f10"):       &bls12381MapG2{},
	common.HexToAddress("0x0100"):       &p256Verify{},
}

func TestPrecompiledP256Verify(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte("vinu latest evm p256"))
	r, s, err := ecdsa.Sign(rand.Reader, key, hash[:])
	if err != nil {
		t.Fatal(err)
	}
	input := make([]byte, 160)
	copy(input[0:32], hash[:])
	r.FillBytes(input[32:64])
	s.FillBytes(input[64:96])
	key.X.FillBytes(input[96:128])
	key.Y.FillBytes(input[128:160])

	p := &p256Verify{}
	if gas := p.RequiredGas(input); gas != 6900 {
		t.Fatalf("P256VERIFY gas = %d, want 6900", gas)
	}
	out, remaining, err := RunPrecompiledContract(p, input, 6900)
	if err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatalf("P256VERIFY remaining gas = %d, want 0", remaining)
	}
	if want := common.LeftPadBytes([]byte{1}, 32); !bytes.Equal(out, want) {
		t.Fatalf("P256VERIFY output = %x, want %x", out, want)
	}

	input[159] ^= 0x01
	out, _, err = RunPrecompiledContract(p, input, 6900)
	if err != nil {
		t.Fatalf("invalid P256VERIFY returned error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("invalid P256VERIFY output = %x, want empty", out)
	}
}

func TestPrecompiledP256VerifyDeterministicEdgeVectors(t *testing.T) {
	input := deterministicP256VerifyInput(t)
	p := &p256Verify{}
	if gas := p.RequiredGas(input); gas != 6900 {
		t.Fatalf("P256VERIFY gas = %d, want 6900", gas)
	}
	out, _, err := RunPrecompiledContract(p, input, 6900)
	if err != nil {
		t.Fatal(err)
	}
	if want := common.LeftPadBytes([]byte{1}, 32); !bytes.Equal(out, want) {
		t.Fatalf("P256VERIFY output = %x, want %x", out, want)
	}

	curve := elliptic.P256()
	curveParams := curve.Params()
	for _, tt := range []struct {
		name   string
		mutate func([]byte)
	}{
		{
			name: "mutated digest",
			mutate: func(in []byte) {
				in[0] ^= 0x01
			},
		},
		{
			name: "zero r",
			mutate: func(in []byte) {
				clear(in[32:64])
			},
		},
		{
			name: "s equals curve order",
			mutate: func(in []byte) {
				curveParams.N.FillBytes(in[64:96])
			},
		},
		{
			name: "qx equals field modulus",
			mutate: func(in []byte) {
				curveParams.P.FillBytes(in[96:128])
			},
		},
		{
			name: "off curve public key",
			mutate: func(in []byte) {
				big.NewInt(1).FillBytes(in[96:128])
				big.NewInt(1).FillBytes(in[128:160])
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			invalid := append([]byte(nil), input...)
			tt.mutate(invalid)
			out, _, err := RunPrecompiledContract(p, invalid, 6900)
			if err != nil {
				t.Fatalf("invalid P256VERIFY returned error: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("invalid P256VERIFY output = %x, want empty", out)
			}
		})
	}
}

func deterministicP256VerifyInput(t *testing.T) []byte {
	t.Helper()
	curve := elliptic.P256()
	curveParams := curve.Params()
	hash := sha256.Sum256([]byte("vinu deterministic p256 vector"))
	qx, qy := curve.ScalarBaseMult([]byte{1})
	rx, _ := curve.ScalarBaseMult([]byte{1})
	r := new(big.Int).Mod(rx, curveParams.N)
	s := new(big.Int).SetBytes(hash[:])
	s.Add(s, r)
	s.Mod(s, curveParams.N)
	if r.Sign() == 0 || s.Sign() == 0 {
		t.Fatal("deterministic P256 vector produced zero signature component")
	}
	if !ecdsa.Verify(&ecdsa.PublicKey{Curve: curve, X: qx, Y: qy}, hash[:], r, s) {
		t.Fatal("deterministic P256 vector does not verify")
	}
	input := make([]byte, 160)
	copy(input[0:32], hash[:])
	r.FillBytes(input[32:64])
	s.FillBytes(input[64:96])
	qx.FillBytes(input[96:128])
	qy.FillBytes(input[128:160])
	return input
}

func TestPrecompiledP256VerifyRejectsMalformedInput(t *testing.T) {
	p := &p256Verify{}
	for _, input := range [][]byte{
		nil,
		make([]byte, 159),
		make([]byte, 161),
		make([]byte, 160),
	} {
		out, _, err := RunPrecompiledContract(p, input, 6900)
		if err != nil {
			t.Fatalf("malformed P256VERIFY returned error: %v", err)
		}
		if len(out) != 0 {
			t.Fatalf("malformed P256VERIFY output = %x, want empty", out)
		}
	}
}

func TestVinuLatestEVMActualPrecompileAddressVectors(t *testing.T) {
	_, _, g1, g2 := bls12381.Generators()
	g1Two := new(bls12381.G1Affine).ScalarMultiplicationBase(big.NewInt(2))
	g1Three := new(bls12381.G1Affine).ScalarMultiplicationBase(big.NewInt(3))
	g1Five := new(bls12381.G1Affine).ScalarMultiplicationBase(big.NewInt(5))
	g2Two := new(bls12381.G2Affine).ScalarMultiplicationBase(big.NewInt(2))
	g2Three := new(bls12381.G2Affine).ScalarMultiplicationBase(big.NewInt(3))
	g2Five := new(bls12381.G2Affine).ScalarMultiplicationBase(big.NewInt(5))

	run := func(name string, addr common.Address, input []byte, want []byte, wantGas uint64) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			p, ok := PrecompiledContractsVinuLatestEVM[addr]
			if !ok {
				t.Fatalf("missing VinuLatestEVM precompile %s", addr)
			}
			if gas := p.RequiredGas(input); gas != wantGas {
				t.Fatalf("%s gas = %d, want %d", name, gas, wantGas)
			}
			out, remaining, err := RunPrecompiledContract(p, input, wantGas)
			if err != nil {
				t.Fatal(err)
			}
			if remaining != 0 {
				t.Fatalf("%s remaining gas = %d, want 0", name, remaining)
			}
			if !bytes.Equal(out, want) {
				t.Fatalf("%s output = %x, want %x", name, out, want)
			}
		})
	}

	g1AddInput := append(encodePointG1(g1Two), encodePointG1(g1Three)...)
	run("BLS12_G1ADD", common.BytesToAddress([]byte{0x0b}), g1AddInput, encodePointG1(g1Five), params.Bls12381G1AddGas)

	g1MSMInput := append(encodePointG1(&g1), common.LeftPadBytes([]byte{2}, 32)...)
	g1MSMInput = append(g1MSMInput, encodePointG1(&g1)...)
	g1MSMInput = append(g1MSMInput, common.LeftPadBytes([]byte{3}, 32)...)
	run("BLS12_G1MSM", common.BytesToAddress([]byte{0x0c}), g1MSMInput, encodePointG1(g1Five), 22776)

	g2AddInput := append(encodePointG2(g2Two), encodePointG2(g2Three)...)
	run("BLS12_G2ADD", common.BytesToAddress([]byte{0x0d}), g2AddInput, encodePointG2(g2Five), params.Bls12381G2AddGas)

	g2MSMInput := append(encodePointG2(&g2), common.LeftPadBytes([]byte{2}, 32)...)
	g2MSMInput = append(g2MSMInput, encodePointG2(&g2)...)
	g2MSMInput = append(g2MSMInput, common.LeftPadBytes([]byte{3}, 32)...)
	run("BLS12_G2MSM", common.BytesToAddress([]byte{0x0e}), g2MSMInput, encodePointG2(g2Five), 45000)

	negG1 := new(bls12381.G1Affine).Neg(&g1)
	pairingInput := append(encodePointG1(&g1), encodePointG2(&g2)...)
	pairingInput = append(pairingInput, encodePointG1(negG1)...)
	pairingInput = append(pairingInput, encodePointG2(&g2)...)
	run("BLS12_PAIRING_CHECK", common.BytesToAddress([]byte{0x0f}), pairingInput, common.LeftPadBytes([]byte{1}, 32), params.Bls12381PairingBaseGas+2*params.Bls12381PairingPerPairGas)

	run("BLS12_MAP_FP_TO_G1", common.BytesToAddress([]byte{0x10}), make([]byte, 64), mustMapToG1(t, make([]byte, 64)), params.Bls12381MapG1Gas)
	run("BLS12_MAP_FP2_TO_G2", common.BytesToAddress([]byte{0x11}), make([]byte, 128), mustMapToG2(t, make([]byte, 128)), params.Bls12381MapG2Gas)

	p256Input := deterministicP256VerifyInput(t)
	run("P256VERIFY", common.BytesToAddress([]byte{0x01, 0x00}), p256Input, common.LeftPadBytes([]byte{1}, 32), 6900)
}

func mustMapToG1(t *testing.T, input []byte) []byte {
	t.Helper()
	out, err := (&bls12381MapG1{}).Run(input)
	if err != nil {
		t.Fatal(err)
	}
	p, err := decodePointG1(out)
	if err != nil {
		t.Fatal(err)
	}
	if !p.IsInSubGroup() {
		t.Fatal("BLS12_MAP_FP_TO_G1 output is not in subgroup")
	}
	return out
}

func mustMapToG2(t *testing.T, input []byte) []byte {
	t.Helper()
	out, err := (&bls12381MapG2{}).Run(input)
	if err != nil {
		t.Fatal(err)
	}
	p, err := decodePointG2(out)
	if err != nil {
		t.Fatal(err)
	}
	if !p.IsInSubGroup() {
		t.Fatal("BLS12_MAP_FP2_TO_G2 output is not in subgroup")
	}
	return out
}

func TestVinuLatestEVMActualBLSPrecompileMalformedInputs(t *testing.T) {
	for _, tt := range []struct {
		name string
		addr common.Address
		in   []byte
		err  error
	}{
		{name: "G1ADD short", addr: common.BytesToAddress([]byte{0x0b}), in: make([]byte, 255), err: errBLS12381InvalidInputLength},
		{name: "G1MSM short", addr: common.BytesToAddress([]byte{0x0c}), in: make([]byte, 159), err: errBLS12381InvalidInputLength},
		{name: "G2ADD short", addr: common.BytesToAddress([]byte{0x0d}), in: make([]byte, 511), err: errBLS12381InvalidInputLength},
		{name: "G2MSM short", addr: common.BytesToAddress([]byte{0x0e}), in: make([]byte, 287), err: errBLS12381InvalidInputLength},
		{name: "pairing short", addr: common.BytesToAddress([]byte{0x0f}), in: make([]byte, 383), err: errBLS12381InvalidInputLength},
		{name: "map G1 short", addr: common.BytesToAddress([]byte{0x10}), in: make([]byte, 63), err: errBLS12381InvalidInputLength},
		{name: "map G2 short", addr: common.BytesToAddress([]byte{0x11}), in: make([]byte, 127), err: errBLS12381InvalidInputLength},
		{name: "map G1 top bytes", addr: common.BytesToAddress([]byte{0x10}), in: leadingNonZeroBytes(64), err: errBLS12381InvalidFieldElementTopBytes},
		{name: "map G2 top bytes", addr: common.BytesToAddress([]byte{0x11}), in: leadingNonZeroBytes(128), err: errBLS12381InvalidFieldElementTopBytes},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := PrecompiledContractsVinuLatestEVM[tt.addr]
			_, _, err := RunPrecompiledContract(p, tt.in, p.RequiredGas(tt.in))
			if err != tt.err {
				t.Fatalf("error = %v, want %v", err, tt.err)
			}
		})
	}
}

func leadingNonZeroBytes(size int) []byte {
	out := make([]byte, size)
	out[0] = 1
	return out
}

func TestPrecompiledModExpVinuLatestEVMGasAndBounds(t *testing.T) {
	input := make([]byte, 96+96)
	big.NewInt(32).FillBytes(input[0:32])
	big.NewInt(32).FillBytes(input[32:64])
	big.NewInt(32).FillBytes(input[64:96])
	big.NewInt(2).FillBytes(input[96:128])
	big.NewInt(2).FillBytes(input[128:160])
	big.NewInt(5).FillBytes(input[160:192])

	berlin := &bigModExp{eip2565: true}
	latest := &bigModExp{eip2565: true, eip7823: true, eip7883: true}
	if gas := berlin.RequiredGas(input); gas != 200 {
		t.Fatalf("EIP-2565 MODEXP gas = %d, want 200", gas)
	}
	if gas := latest.RequiredGas(input); gas != 500 {
		t.Fatalf("EIP-7883 MODEXP gas = %d, want 500", gas)
	}
	out, _, err := RunPrecompiledContract(latest, input, 500)
	if err != nil {
		t.Fatal(err)
	}
	if want := common.LeftPadBytes([]byte{4}, 32); !bytes.Equal(out, want) {
		t.Fatalf("MODEXP output = %x, want %x", out, want)
	}

	oversize := make([]byte, 96)
	big.NewInt(1025).FillBytes(oversize[0:32])
	big.NewInt(1).FillBytes(oversize[32:64])
	big.NewInt(1).FillBytes(oversize[64:96])
	if gas := latest.RequiredGas(oversize); gas != math.MaxUint64 {
		t.Fatalf("oversized MODEXP gas = %d, want MaxUint64", gas)
	}
	_, _, err = RunPrecompiledContract(latest, oversize, math.MaxUint64)
	if err != errModExpInputTooLarge {
		t.Fatalf("oversized MODEXP error = %v, want %v", err, errModExpInputTooLarge)
	}
}

func TestPrecompiledModExpVinuLatestEVMBoundaryVectors(t *testing.T) {
	latest := &bigModExp{eip2565: true, eip7823: true, eip7883: true}
	for _, tt := range []struct {
		name             string
		baseLen, expLen  int
		modLen           int
		base, exp, mod   *big.Int
		wantGas          uint64
		wantResultLength int
	}{
		{
			name:             "zero base and modulus length returns empty",
			baseLen:          0,
			expLen:           0,
			modLen:           0,
			base:             big.NewInt(0),
			exp:              big.NewInt(0),
			mod:              big.NewInt(0),
			wantGas:          500,
			wantResultLength: 0,
		},
		{
			name:             "one hundred twenty eight byte operands exceed minimum gas",
			baseLen:          128,
			expLen:           1,
			modLen:           128,
			base:             big.NewInt(2),
			exp:              big.NewInt(1),
			mod:              big.NewInt(17),
			wantGas:          512,
			wantResultLength: 128,
		},
		{
			name:             "thirty three byte exponent uses sixteen bit adjustment",
			baseLen:          32,
			expLen:           33,
			modLen:           32,
			base:             big.NewInt(2),
			exp:              new(big.Int).Lsh(big.NewInt(1), 263),
			mod:              big.NewInt(17),
			wantGas:          4336,
			wantResultLength: 32,
		},
		{
			name:             "one thousand twenty four byte operands are accepted",
			baseLen:          1024,
			expLen:           0,
			modLen:           1024,
			base:             big.NewInt(2),
			exp:              big.NewInt(0),
			mod:              big.NewInt(17),
			wantGas:          32768,
			wantResultLength: 1024,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			input := makeModExpInput(tt.baseLen, tt.expLen, tt.modLen, tt.base, tt.exp, tt.mod)
			if gas := latest.RequiredGas(input); gas != tt.wantGas {
				t.Fatalf("MODEXP gas = %d, want %d", gas, tt.wantGas)
			}
			out, remaining, err := RunPrecompiledContract(latest, input, tt.wantGas)
			if err != nil {
				t.Fatal(err)
			}
			if remaining != 0 {
				t.Fatalf("remaining gas = %d, want 0", remaining)
			}
			if len(out) != tt.wantResultLength {
				t.Fatalf("MODEXP output length = %d, want %d", len(out), tt.wantResultLength)
			}
			want := modExpExpected(tt.modLen, tt.base, tt.exp, tt.mod)
			if !bytes.Equal(out, want) {
				t.Fatalf("MODEXP output = %x, want %x", out, want)
			}
		})
	}
}

func TestPrecompiledModExpVinuLatestEVMRejectsEachOversizedOperand(t *testing.T) {
	latest := &bigModExp{eip2565: true, eip7823: true, eip7883: true}
	for _, tt := range []struct {
		name           string
		base, exp, mod int
	}{
		{name: "base length", base: 1025, exp: 1, mod: 1},
		{name: "exponent length", base: 1, exp: 1025, mod: 1},
		{name: "modulus length", base: 1, exp: 1, mod: 1025},
	} {
		t.Run(tt.name, func(t *testing.T) {
			input := makeModExpInput(tt.base, tt.exp, tt.mod, big.NewInt(1), big.NewInt(1), big.NewInt(1))
			if gas := latest.RequiredGas(input); gas != math.MaxUint64 {
				t.Fatalf("oversized MODEXP gas = %d, want MaxUint64", gas)
			}
			_, _, err := RunPrecompiledContract(latest, input, math.MaxUint64)
			if err != errModExpInputTooLarge {
				t.Fatalf("oversized MODEXP error = %v, want %v", err, errModExpInputTooLarge)
			}
		})
	}
}

func makeModExpInput(baseLen, expLen, modLen int, base, exp, mod *big.Int) []byte {
	input := make([]byte, 96+baseLen+expLen+modLen)
	big.NewInt(int64(baseLen)).FillBytes(input[0:32])
	big.NewInt(int64(expLen)).FillBytes(input[32:64])
	big.NewInt(int64(modLen)).FillBytes(input[64:96])
	fillBigIntBytes(input[96:96+baseLen], base)
	fillBigIntBytes(input[96+baseLen:96+baseLen+expLen], exp)
	fillBigIntBytes(input[96+baseLen+expLen:], mod)
	return input
}

func fillBigIntBytes(dst []byte, n *big.Int) {
	if len(dst) == 0 || n == nil {
		return
	}
	n.FillBytes(dst)
}

func modExpExpected(modLen int, base, exp, mod *big.Int) []byte {
	if mod.Sign() == 0 {
		return make([]byte, modLen)
	}
	return common.LeftPadBytes(new(big.Int).Exp(base, exp, mod).Bytes(), modLen)
}

func TestVinuLatestEVMActivePrecompiles(t *testing.T) {
	p256Addr := common.BytesToAddress([]byte{0x01, 0x00})
	if _, ok := PrecompiledContractsBLS[p256Addr]; ok {
		t.Fatal("P256VERIFY must not be active in the BLS-only precompile set")
	}
	if _, ok := PrecompiledContractsVinuLatestEVM[p256Addr]; !ok {
		t.Fatal("P256VERIFY missing from VinuLatestEVM precompile set")
	}
	if _, ok := PrecompiledContractsVinuLatestEVM[common.BytesToAddress([]byte{5})].(*bigModExp); !ok {
		t.Fatal("MODEXP missing from VinuLatestEVM precompile set")
	}

	active := make(map[common.Address]struct{})
	for _, addr := range ActivePrecompiles(params.Rules{IsBerlin: true, IsVinuBLS: true, IsVinuLatestEVM: true}) {
		active[addr] = struct{}{}
	}
	if _, ok := active[p256Addr]; !ok {
		t.Fatal("P256VERIFY missing from active VinuLatestEVM precompiles")
	}
	if _, ok := active[common.BytesToAddress([]byte{0x0b})]; !ok {
		t.Fatal("BLS12_G1ADD missing from active VinuLatestEVM precompiles")
	}
}

// EIP-152 test vectors
var blake2FMalformedInputTests = []precompiledFailureTest{
	{
		Input:         "",
		ExpectedError: errBlake2FInvalidInputLength.Error(),
		Name:          "vector 0: empty input",
	},
	{
		Input:         "00000c48c9bdf267e6096a3ba7ca8485ae67bb2bf894fe72f36e3cf1361d5f3af54fa5d182e6ad7f520e511f6c3e2b8c68059b6bbd41fbabd9831f79217e1319cde05b61626300000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000300000000000000000000000000000001",
		ExpectedError: errBlake2FInvalidInputLength.Error(),
		Name:          "vector 1: less than 213 bytes input",
	},
	{
		Input:         "000000000c48c9bdf267e6096a3ba7ca8485ae67bb2bf894fe72f36e3cf1361d5f3af54fa5d182e6ad7f520e511f6c3e2b8c68059b6bbd41fbabd9831f79217e1319cde05b61626300000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000300000000000000000000000000000001",
		ExpectedError: errBlake2FInvalidInputLength.Error(),
		Name:          "vector 2: more than 213 bytes input",
	},
	{
		Input:         "0000000c48c9bdf267e6096a3ba7ca8485ae67bb2bf894fe72f36e3cf1361d5f3af54fa5d182e6ad7f520e511f6c3e2b8c68059b6bbd41fbabd9831f79217e1319cde05b61626300000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000300000000000000000000000000000002",
		ExpectedError: errBlake2FInvalidFinalFlag.Error(),
		Name:          "vector 3: malformed final block indicator flag",
	},
}

func testPrecompiled(addr string, test precompiledTest, t *testing.T) {
	p := allPrecompiles[common.HexToAddress(addr)]
	in := common.Hex2Bytes(test.Input)
	gas := p.RequiredGas(in)
	t.Run(fmt.Sprintf("%s-Gas=%d", test.Name, gas), func(t *testing.T) {
		if res, _, err := RunPrecompiledContract(p, in, gas); err != nil {
			t.Error(err)
		} else if common.Bytes2Hex(res) != test.Expected {
			t.Errorf("Expected %v, got %v", test.Expected, common.Bytes2Hex(res))
		}
		if expGas := test.Gas; expGas != gas {
			t.Errorf("%v: gas wrong, expected %d, got %d", test.Name, expGas, gas)
		}
		// Verify that the precompile did not touch the input buffer
		exp := common.Hex2Bytes(test.Input)
		if !bytes.Equal(in, exp) {
			t.Errorf("Precompiled %v modified input data", addr)
		}
	})
}

func testPrecompiledOOG(addr string, test precompiledTest, t *testing.T) {
	p := allPrecompiles[common.HexToAddress(addr)]
	in := common.Hex2Bytes(test.Input)
	gas := p.RequiredGas(in) - 1

	t.Run(fmt.Sprintf("%s-Gas=%d", test.Name, gas), func(t *testing.T) {
		_, _, err := RunPrecompiledContract(p, in, gas)
		if err.Error() != "out of gas" {
			t.Errorf("Expected error [out of gas], got [%v]", err)
		}
		// Verify that the precompile did not touch the input buffer
		exp := common.Hex2Bytes(test.Input)
		if !bytes.Equal(in, exp) {
			t.Errorf("Precompiled %v modified input data", addr)
		}
	})
}

func testPrecompiledFailure(addr string, test precompiledFailureTest, t *testing.T) {
	p := allPrecompiles[common.HexToAddress(addr)]
	in := common.Hex2Bytes(test.Input)
	gas := p.RequiredGas(in)
	t.Run(test.Name, func(t *testing.T) {
		_, _, err := RunPrecompiledContract(p, in, gas)
		if err.Error() != test.ExpectedError {
			t.Errorf("Expected error [%v], got [%v]", test.ExpectedError, err)
		}
		// Verify that the precompile did not touch the input buffer
		exp := common.Hex2Bytes(test.Input)
		if !bytes.Equal(in, exp) {
			t.Errorf("Precompiled %v modified input data", addr)
		}
	})
}

func benchmarkPrecompiled(addr string, test precompiledTest, bench *testing.B) {
	if test.NoBenchmark {
		return
	}
	p := allPrecompiles[common.HexToAddress(addr)]
	in := common.Hex2Bytes(test.Input)
	reqGas := p.RequiredGas(in)

	var (
		res  []byte
		err  error
		data = make([]byte, len(in))
	)

	bench.Run(fmt.Sprintf("%s-Gas=%d", test.Name, reqGas), func(bench *testing.B) {
		bench.ReportAllocs()
		start := time.Now()
		bench.ResetTimer()
		for i := 0; i < bench.N; i++ {
			copy(data, in)
			res, _, err = RunPrecompiledContract(p, data, reqGas)
		}
		bench.StopTimer()
		elapsed := uint64(time.Since(start))
		if elapsed < 1 {
			elapsed = 1
		}
		gasUsed := reqGas * uint64(bench.N)
		bench.ReportMetric(float64(reqGas), "gas/op")
		// Keep it as uint64, multiply 100 to get two digit float later
		mgasps := (100 * 1000 * gasUsed) / elapsed
		bench.ReportMetric(float64(mgasps)/100, "mgas/s")
		//Check if it is correct
		if err != nil {
			bench.Error(err)
			return
		}
		if common.Bytes2Hex(res) != test.Expected {
			bench.Errorf("Expected %v, got %v", test.Expected, common.Bytes2Hex(res))
			return
		}
	})
}

// Benchmarks the sample inputs from the ECRECOVER precompile.
func BenchmarkPrecompiledEcrecover(bench *testing.B) {
	t := precompiledTest{
		Input:    "38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e000000000000000000000000000000000000000000000000000000000000001b38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e789d1dd423d25f0772d2748d60f7e4b81bb14d086eba8e8e8efb6dcff8a4ae02",
		Expected: "000000000000000000000000ceaccac640adf55b2028469bd36ba501f28b699d",
		Name:     "",
	}
	benchmarkPrecompiled("01", t, bench)
}

// Benchmarks the sample inputs from the SHA256 precompile.
func BenchmarkPrecompiledSha256(bench *testing.B) {
	t := precompiledTest{
		Input:    "38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e000000000000000000000000000000000000000000000000000000000000001b38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e789d1dd423d25f0772d2748d60f7e4b81bb14d086eba8e8e8efb6dcff8a4ae02",
		Expected: "811c7003375852fabd0d362e40e68607a12bdabae61a7d068fe5fdd1dbbf2a5d",
		Name:     "128",
	}
	benchmarkPrecompiled("02", t, bench)
}

// Benchmarks the sample inputs from the RIPEMD precompile.
func BenchmarkPrecompiledRipeMD(bench *testing.B) {
	t := precompiledTest{
		Input:    "38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e000000000000000000000000000000000000000000000000000000000000001b38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e789d1dd423d25f0772d2748d60f7e4b81bb14d086eba8e8e8efb6dcff8a4ae02",
		Expected: "0000000000000000000000009215b8d9882ff46f0dfde6684d78e831467f65e6",
		Name:     "128",
	}
	benchmarkPrecompiled("03", t, bench)
}

// Benchmarks the sample inputs from the identiy precompile.
func BenchmarkPrecompiledIdentity(bench *testing.B) {
	t := precompiledTest{
		Input:    "38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e000000000000000000000000000000000000000000000000000000000000001b38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e789d1dd423d25f0772d2748d60f7e4b81bb14d086eba8e8e8efb6dcff8a4ae02",
		Expected: "38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e000000000000000000000000000000000000000000000000000000000000001b38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e789d1dd423d25f0772d2748d60f7e4b81bb14d086eba8e8e8efb6dcff8a4ae02",
		Name:     "128",
	}
	benchmarkPrecompiled("04", t, bench)
}

// Tests the sample inputs from the ModExp EIP 198.
func TestPrecompiledModExp(t *testing.T)      { testJson("modexp", "05", t) }
func BenchmarkPrecompiledModExp(b *testing.B) { benchJson("modexp", "05", b) }

func TestPrecompiledModExpEip2565(t *testing.T)      { testJson("modexp_eip2565", "f5", t) }
func BenchmarkPrecompiledModExpEip2565(b *testing.B) { benchJson("modexp_eip2565", "f5", b) }

// Tests the sample inputs from the elliptic curve addition EIP 213.
func TestPrecompiledBn256Add(t *testing.T)      { testJson("bn256Add", "06", t) }
func BenchmarkPrecompiledBn256Add(b *testing.B) { benchJson("bn256Add", "06", b) }

// Tests OOG
func TestPrecompiledModExpOOG(t *testing.T) {
	modexpTests, err := loadJson("modexp")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range modexpTests {
		testPrecompiledOOG("05", test, t)
	}
}

// Tests the sample inputs from the elliptic curve scalar multiplication EIP 213.
func TestPrecompiledBn256ScalarMul(t *testing.T)      { testJson("bn256ScalarMul", "07", t) }
func BenchmarkPrecompiledBn256ScalarMul(b *testing.B) { benchJson("bn256ScalarMul", "07", b) }

// Tests the sample inputs from the elliptic curve pairing check EIP 197.
func TestPrecompiledBn256Pairing(t *testing.T)      { testJson("bn256Pairing", "08", t) }
func BenchmarkPrecompiledBn256Pairing(b *testing.B) { benchJson("bn256Pairing", "08", b) }

func TestPrecompiledBlake2F(t *testing.T)      { testJson("blake2F", "09", t) }
func BenchmarkPrecompiledBlake2F(b *testing.B) { benchJson("blake2F", "09", b) }

func TestPrecompileBlake2FMalformedInput(t *testing.T) {
	for _, test := range blake2FMalformedInputTests {
		testPrecompiledFailure("09", test, t)
	}
}

func TestPrecompiledEcrecover(t *testing.T) { testJson("ecRecover", "01", t) }

func testJson(name, addr string, t *testing.T) {
	tests, err := loadJson(name)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		testPrecompiled(addr, test, t)
	}
}

func testJsonFail(name, addr string, t *testing.T) {
	tests, err := loadJsonFail(name)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		testPrecompiledFailure(addr, test, t)
	}
}

func benchJson(name, addr string, b *testing.B) {
	tests, err := loadJson(name)
	if err != nil {
		b.Fatal(err)
	}
	for _, test := range tests {
		benchmarkPrecompiled(addr, test, b)
	}
}

func TestPrecompiledBLS12381G1Add(t *testing.T)      { testJson("blsG1Add", "0f0a", t) }
func TestPrecompiledBLS12381G1MultiExp(t *testing.T) { testJson("blsG1MultiExp", "0f0b", t) }
func TestPrecompiledBLS12381G2Add(t *testing.T)      { testJson("blsG2Add", "0f0c", t) }
func TestPrecompiledBLS12381G2MultiExp(t *testing.T) { testJson("blsG2MultiExp", "0f0d", t) }
func TestPrecompiledBLS12381Pairing(t *testing.T)    { testJson("blsPairing", "0f0e", t) }
func TestPrecompiledBLS12381MapG1(t *testing.T)      { testJson("blsMapG1", "0f0f", t) }
func TestPrecompiledBLS12381MapG2(t *testing.T)      { testJson("blsMapG2", "0f10", t) }

// Upstream keeps these fixture filenames for single-term MSM coverage. Final
// EIP-2537 exposes no standalone G1Mul/G2Mul precompile.
func TestPrecompiledBLS12381G1MSMSingleTerm(t *testing.T) { testJson("blsG1Mul", "0f0b", t) }
func TestPrecompiledBLS12381G2MSMSingleTerm(t *testing.T) { testJson("blsG2Mul", "0f0d", t) }

func TestPrecompiledBLS12381ActiveAddresses(t *testing.T) {
	active := make(map[common.Address]bool)
	for _, addr := range ActivePrecompiles(params.Rules{IsBerlin: true, IsVinuBLS: true}) {
		active[addr] = true
	}
	for _, addr := range []common.Address{
		common.BytesToAddress([]byte{0x01}),
		common.BytesToAddress([]byte{0x02}),
		common.BytesToAddress([]byte{0x03}),
		common.BytesToAddress([]byte{0x04}),
		common.BytesToAddress([]byte{0x05}),
		common.BytesToAddress([]byte{0x06}),
		common.BytesToAddress([]byte{0x07}),
		common.BytesToAddress([]byte{0x08}),
		common.BytesToAddress([]byte{0x09}),
		common.BytesToAddress([]byte{0x0b}),
		common.BytesToAddress([]byte{0x0c}),
		common.BytesToAddress([]byte{0x0d}),
		common.BytesToAddress([]byte{0x0e}),
		common.BytesToAddress([]byte{0x0f}),
		common.BytesToAddress([]byte{0x10}),
		common.BytesToAddress([]byte{0x11}),
	} {
		if !active[addr] {
			t.Fatalf("expected BLS active precompile %s", addr)
		}
		if _, ok := PrecompiledContractsBLS[addr]; !ok {
			t.Fatalf("expected BLS precompile map entry %s", addr)
		}
	}
	for _, addr := range []common.Address{
		common.BytesToAddress([]byte{0x0a}),
		common.BytesToAddress([]byte{0x12}),
	} {
		if active[addr] {
			t.Fatalf("unexpected BLS active precompile %s", addr)
		}
		if _, ok := PrecompiledContractsBLS[addr]; ok {
			t.Fatalf("unexpected BLS precompile map entry %s", addr)
		}
	}

	berlinOnly := make(map[common.Address]bool)
	for _, addr := range ActivePrecompiles(params.Rules{IsBerlin: true}) {
		berlinOnly[addr] = true
	}
	if berlinOnly[common.BytesToAddress([]byte{0x0b})] {
		t.Fatal("BLS precompile active without VinuBLS rule")
	}
}

func BenchmarkPrecompiledBLS12381G1Add(b *testing.B)      { benchJson("blsG1Add", "0f0a", b) }
func BenchmarkPrecompiledBLS12381G1MultiExp(b *testing.B) { benchJson("blsG1MultiExp", "0f0b", b) }
func BenchmarkPrecompiledBLS12381G2Add(b *testing.B)      { benchJson("blsG2Add", "0f0c", b) }
func BenchmarkPrecompiledBLS12381G2MultiExp(b *testing.B) { benchJson("blsG2MultiExp", "0f0d", b) }
func BenchmarkPrecompiledBLS12381Pairing(b *testing.B)    { benchJson("blsPairing", "0f0e", b) }
func BenchmarkPrecompiledBLS12381MapG1(b *testing.B)      { benchJson("blsMapG1", "0f0f", b) }
func BenchmarkPrecompiledBLS12381MapG2(b *testing.B)      { benchJson("blsMapG2", "0f10", b) }
func BenchmarkPrecompiledBLS12381G1MSMSingleTerm(b *testing.B) {
	benchJson("blsG1Mul", "0f0b", b)
}
func BenchmarkPrecompiledBLS12381G2MSMSingleTerm(b *testing.B) {
	benchJson("blsG2Mul", "0f0d", b)
}

// Failure tests
func TestPrecompiledBLS12381G1AddFail(t *testing.T)      { testJsonFail("blsG1Add", "0f0a", t) }
func TestPrecompiledBLS12381G1MultiExpFail(t *testing.T) { testJsonFail("blsG1MultiExp", "0f0b", t) }
func TestPrecompiledBLS12381G2AddFail(t *testing.T)      { testJsonFail("blsG2Add", "0f0c", t) }
func TestPrecompiledBLS12381G2MultiExpFail(t *testing.T) { testJsonFail("blsG2MultiExp", "0f0d", t) }
func TestPrecompiledBLS12381PairingFail(t *testing.T)    { testJsonFail("blsPairing", "0f0e", t) }
func TestPrecompiledBLS12381MapG1Fail(t *testing.T)      { testJsonFail("blsMapG1", "0f0f", t) }
func TestPrecompiledBLS12381MapG2Fail(t *testing.T)      { testJsonFail("blsMapG2", "0f10", t) }
func TestPrecompiledBLS12381G1MSMSingleTermFail(t *testing.T) {
	testJsonFail("blsG1Mul", "0f0b", t)
}
func TestPrecompiledBLS12381G2MSMSingleTermFail(t *testing.T) {
	testJsonFail("blsG2Mul", "0f0d", t)
}

func loadJson(name string) ([]precompiledTest, error) {
	data, err := ioutil.ReadFile(fmt.Sprintf("testdata/precompiles/%v.json", name))
	if err != nil {
		return nil, err
	}
	var testcases []precompiledTest
	err = json.Unmarshal(data, &testcases)
	return testcases, err
}

func loadJsonFail(name string) ([]precompiledFailureTest, error) {
	data, err := ioutil.ReadFile(fmt.Sprintf("testdata/precompiles/fail-%v.json", name))
	if err != nil {
		return nil, err
	}
	var testcases []precompiledFailureTest
	err = json.Unmarshal(data, &testcases)
	return testcases, err
}

// BenchmarkPrecompiledBLS12381G1MultiExpWorstCase benchmarks the worst case we could find that still fits a gaslimit of 10MGas.
func BenchmarkPrecompiledBLS12381G1MultiExpWorstCase(b *testing.B) {
	task := "0000000000000000000000000000000008d8c4a16fb9d8800cce987c0eadbb6b3b005c213d44ecb5adeed713bae79d606041406df26169c35df63cf972c94be1" +
		"0000000000000000000000000000000011bc8afe71676e6730702a46ef817060249cd06cd82e6981085012ff6d013aa4470ba3a2c71e13ef653e1e223d1ccfe9" +
		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"
	input := task
	for i := 0; i < 4787; i++ {
		input = input + task
	}
	testcase := precompiledTest{
		Input:       input,
		Expected:    "0000000000000000000000000000000005a6310ea6f2a598023ae48819afc292b4dfcb40aabad24a0c2cb6c19769465691859eeb2a764342a810c5038d700f18000000000000000000000000000000001268ac944437d15923dc0aec00daa9250252e43e4b35ec7a19d01f0d6cd27f6e139d80dae16ba1c79cc7f57055a93ff5",
		Name:        "WorstCaseG1",
		NoBenchmark: false,
	}
	benchmarkPrecompiled("0c", testcase, b)
}

// BenchmarkPrecompiledBLS12381G2MultiExpWorstCase benchmarks the worst case we could find that still fits a gaslimit of 10MGas.
func BenchmarkPrecompiledBLS12381G2MultiExpWorstCase(b *testing.B) {
	task := "000000000000000000000000000000000d4f09acd5f362e0a516d4c13c5e2f504d9bd49fdfb6d8b7a7ab35a02c391c8112b03270d5d9eefe9b659dd27601d18f" +
		"000000000000000000000000000000000fd489cb75945f3b5ebb1c0e326d59602934c8f78fe9294a8877e7aeb95de5addde0cb7ab53674df8b2cfbb036b30b99" +
		"00000000000000000000000000000000055dbc4eca768714e098bbe9c71cf54b40f51c26e95808ee79225a87fb6fa1415178db47f02d856fea56a752d185f86b" +
		"000000000000000000000000000000001239b7640f416eb6e921fe47f7501d504fadc190d9cf4e89ae2b717276739a2f4ee9f637c35e23c480df029fd8d247c7" +
		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"
	input := task
	for i := 0; i < 1040; i++ {
		input = input + task
	}

	testcase := precompiledTest{
		Input:       input,
		Expected:    "0000000000000000000000000000000018f5ea0c8b086095cfe23f6bb1d90d45de929292006dba8cdedd6d3203af3c6bbfd592e93ecb2b2c81004961fdcbb46c00000000000000000000000000000000076873199175664f1b6493a43c02234f49dc66f077d3007823e0343ad92e30bd7dc209013435ca9f197aca44d88e9dac000000000000000000000000000000000e6f07f4b23b511eac1e2682a0fc224c15d80e122a3e222d00a41fab15eba645a700b9ae84f331ae4ed873678e2e6c9b000000000000000000000000000000000bcb4849e460612aaed79617255fd30c03f51cf03d2ed4163ca810c13e1954b1e8663157b957a601829bb272a4e6c7b8",
		Name:        "WorstCaseG2",
		NoBenchmark: false,
	}
	benchmarkPrecompiled("0f", testcase, b)
}
