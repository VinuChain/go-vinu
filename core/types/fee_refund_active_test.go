// Copyright 2026 The go-vinu Authors
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

package types

import (
	"bytes"
	"math/big"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/rlp"
)

// saveAndRestoreFeeRefundActive snapshots the global FeeRefundActive atomic
// and returns a cleanup function the caller should defer. Tests in this file
// mutate the flag; the snapshot/restore pattern prevents cross-test leakage.
func saveAndRestoreFeeRefundActive(t *testing.T) {
	t.Helper()
	prev := FeeRefundActive.Load()
	t.Cleanup(func() { FeeRefundActive.Store(prev) })
}

// encodeReceiptBypassingFlag encodes a receiptRLP directly, skipping
// Receipt.EncodeRLP which zeroes FeeRefund when FeeRefundActive is false.
// This lets tests put arbitrary FeeRefund bytes on the wire regardless of
// the flag state, simulating what a post-Podgorica peer would send.
func encodeReceiptBypassingFlag(t *testing.T, refund *big.Int) []byte {
	t.Helper()
	raw, err := rlp.EncodeToBytes(&receiptRLP{
		PostStateOrStatus: receiptStatusSuccessfulRLP,
		CumulativeGasUsed: 1,
		FeeRefund:         refund,
		Bloom:             Bloom{},
		Logs:              nil,
	})
	if err != nil {
		t.Fatalf("encode setup: %v", err)
	}
	return raw
}

// --- Encode: flag state controls wire bytes -------------------------------

func TestEncode_FeeRefundStrippedWhenActiveFalse(t *testing.T) {
	saveAndRestoreFeeRefundActive(t)
	FeeRefundActive.Store(false)

	r := &Receipt{
		Status:            ReceiptStatusSuccessful,
		CumulativeGasUsed: 21000,
		FeeRefund:         big.NewInt(987_654_321),
	}
	var buf bytes.Buffer
	if err := r.EncodeRLP(&buf); err != nil {
		t.Fatalf("EncodeRLP: %v", err)
	}

	var wire receiptRLP
	if err := rlp.DecodeBytes(buf.Bytes(), &wire); err != nil {
		t.Fatalf("decode wire: %v", err)
	}
	if wire.FeeRefund == nil {
		t.Fatal("wire FeeRefund must be a concrete zero big.Int, not nil")
	}
	if wire.FeeRefund.Sign() != 0 {
		t.Fatalf("pre-Podgorica encoding must strip FeeRefund; got %s", wire.FeeRefund)
	}
}

func TestEncode_FeeRefundRoundTripsWhenActiveTrue(t *testing.T) {
	saveAndRestoreFeeRefundActive(t)
	FeeRefundActive.Store(true)

	original := big.NewInt(987_654_321)
	r := &Receipt{
		Status:            ReceiptStatusSuccessful,
		CumulativeGasUsed: 21000,
		FeeRefund:         new(big.Int).Set(original),
	}
	var buf bytes.Buffer
	if err := r.EncodeRLP(&buf); err != nil {
		t.Fatalf("EncodeRLP: %v", err)
	}

	var decoded Receipt
	if err := rlp.DecodeBytes(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.FeeRefund == nil {
		t.Fatal("FeeRefund must be set after post-Podgorica round-trip")
	}
	if decoded.FeeRefund.Cmp(original) != 0 {
		t.Fatalf("FeeRefund round-trip mismatch: got %s, want %s", decoded.FeeRefund, original)
	}
}

// --- Decode: flag state controls whether wire value is accepted ----------

func TestDecode_FeeRefundDroppedWhenActiveFalse(t *testing.T) {
	saveAndRestoreFeeRefundActive(t)
	FeeRefundActive.Store(false)

	wire := encodeReceiptBypassingFlag(t, big.NewInt(42))
	var decoded Receipt
	if err := rlp.DecodeBytes(wire, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.FeeRefund != nil && decoded.FeeRefund.Sign() != 0 {
		t.Fatalf("expected FeeRefund to be dropped pre-Podgorica; got %s", decoded.FeeRefund)
	}
}

func TestDecode_FeeRefundAcceptedWhenActiveTrue(t *testing.T) {
	saveAndRestoreFeeRefundActive(t)
	FeeRefundActive.Store(true)

	wire := encodeReceiptBypassingFlag(t, big.NewInt(42))
	var decoded Receipt
	if err := rlp.DecodeBytes(wire, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.FeeRefund == nil {
		t.Fatal("FeeRefund must be set post-Podgorica")
	}
	if decoded.FeeRefund.Int64() != 42 {
		t.Fatalf("expected FeeRefund=42, got %s", decoded.FeeRefund)
	}
}

// --- Transition: state at operation time is what matters -----------------

// TestFlagTransition_EncodedWhileFalseDecodedWhileTrue verifies that a
// receipt encoded while the flag is false has no FeeRefund even when later
// decoded while the flag is true. The state at encode time is binding.
func TestFlagTransition_EncodedWhileFalseDecodedWhileTrue(t *testing.T) {
	saveAndRestoreFeeRefundActive(t)

	FeeRefundActive.Store(false)
	r := &Receipt{
		Status:            ReceiptStatusSuccessful,
		CumulativeGasUsed: 21000,
		FeeRefund:         big.NewInt(5_555),
	}
	var buf bytes.Buffer
	if err := r.EncodeRLP(&buf); err != nil {
		t.Fatalf("encode: %v", err)
	}

	// Flip to post-Podgorica before decoding.
	FeeRefundActive.Store(true)

	var decoded Receipt
	if err := rlp.DecodeBytes(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.FeeRefund == nil {
		t.Fatal("decoded FeeRefund must not be nil when flag is true at decode")
	}
	if decoded.FeeRefund.Sign() != 0 {
		t.Fatalf("flag was false at encode, so wire value is zero; got %s", decoded.FeeRefund)
	}
}

// TestFlagTransition_EncodedWhileTrueDecodedWhileFalse verifies that the
// reverse transition discards the nonzero wire value: a receipt encoded
// while the flag is true, then decoded while the flag is false, must come
// back with FeeRefund suppressed so pre-Podgorica hash determinism holds.
func TestFlagTransition_EncodedWhileTrueDecodedWhileFalse(t *testing.T) {
	saveAndRestoreFeeRefundActive(t)

	FeeRefundActive.Store(true)
	r := &Receipt{
		Status:            ReceiptStatusSuccessful,
		CumulativeGasUsed: 21000,
		FeeRefund:         big.NewInt(9_999),
	}
	var buf bytes.Buffer
	if err := r.EncodeRLP(&buf); err != nil {
		t.Fatalf("encode: %v", err)
	}

	FeeRefundActive.Store(false)

	var decoded Receipt
	if err := rlp.DecodeBytes(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.FeeRefund != nil && decoded.FeeRefund.Sign() != 0 {
		t.Fatalf("flag was false at decode, wire FeeRefund must be dropped; got %s",
			decoded.FeeRefund)
	}
}

// --- Nil FeeRefund handling across flag states ---------------------------

func TestEncode_NilFeeRefund_ProducesZeroWire_ActiveFalse(t *testing.T) {
	saveAndRestoreFeeRefundActive(t)
	FeeRefundActive.Store(false)

	r := &Receipt{
		Status:            ReceiptStatusSuccessful,
		CumulativeGasUsed: 21000,
		FeeRefund:         nil,
	}
	var buf bytes.Buffer
	if err := r.EncodeRLP(&buf); err != nil {
		t.Fatalf("encode: %v", err)
	}
	var wire receiptRLP
	if err := rlp.DecodeBytes(buf.Bytes(), &wire); err != nil {
		t.Fatalf("decode wire: %v", err)
	}
	if wire.FeeRefund == nil || wire.FeeRefund.Sign() != 0 {
		t.Fatalf("nil FeeRefund must encode as zero pre-Podgorica; got %v", wire.FeeRefund)
	}
}

func TestEncode_NilFeeRefund_ProducesZeroWire_ActiveTrue(t *testing.T) {
	saveAndRestoreFeeRefundActive(t)
	FeeRefundActive.Store(true)

	r := &Receipt{
		Status:            ReceiptStatusSuccessful,
		CumulativeGasUsed: 21000,
		FeeRefund:         nil,
	}
	var buf bytes.Buffer
	if err := r.EncodeRLP(&buf); err != nil {
		t.Fatalf("encode: %v", err)
	}
	var wire receiptRLP
	if err := rlp.DecodeBytes(buf.Bytes(), &wire); err != nil {
		t.Fatalf("decode wire: %v", err)
	}
	if wire.FeeRefund == nil || wire.FeeRefund.Sign() != 0 {
		t.Fatalf("nil FeeRefund must encode as zero post-Podgorica too; got %v", wire.FeeRefund)
	}
}

// --- safeFeeRefund helpers -----------------------------------------------

func TestSafeFeeRefund_NilInput(t *testing.T) {
	got := safeFeeRefund(nil)
	if got == nil {
		t.Fatal("safeFeeRefund(nil) must return a zero-valued big.Int, not nil")
	}
	if got.Sign() != 0 {
		t.Fatalf("safeFeeRefund(nil) must be zero; got %s", got)
	}
}

func TestSafeFeeRefund_CopiesValue(t *testing.T) {
	original := big.NewInt(1_000)
	got := safeFeeRefund(original)
	if got == original {
		t.Fatal("safeFeeRefund must return a copy, not share the input pointer")
	}
	if got.Cmp(original) != 0 {
		t.Fatalf("safeFeeRefund copy mismatch: got %s, want %s", got, original)
	}
	got.Add(got, big.NewInt(1))
	if original.Int64() != 1_000 {
		t.Fatal("mutating the copy must not mutate the original")
	}
}

// --- Concurrent flag flip does not race on the getter -------------------

// TestFlagFlipRaceFree stresses concurrent loads from feeRefundForEncoding
// while a writer toggles the flag. atomic.Bool semantics mean no data race;
// this test exists to fail under -race if anyone replaces the atomic with
// a plain bool during future refactors.
func TestFlagFlipRaceFree(t *testing.T) {
	saveAndRestoreFeeRefundActive(t)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		toggle := false
		for {
			select {
			case <-stop:
				return
			default:
				toggle = !toggle
				FeeRefundActive.Store(toggle)
			}
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 10_000; i++ {
			_ = feeRefundForEncoding(big.NewInt(int64(i)))
		}
		close(stop)
	}()

	wg.Wait()
}
