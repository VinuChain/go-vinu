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

package snap

import (
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/rlp"
)

func TestDecodeResponseListLimits(t *testing.T) {
	want := [][]byte{{1}, {2}}
	raw, err := rlp.EncodeToBytes(want)
	if err != nil {
		t.Fatal(err)
	}
	var got [][]byte
	if err := decodeResponseList(raw, len(want), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("decoded %d items, want %d", len(got), len(want))
	}

	tooMany, err := rlp.EncodeToBytes(make([][]byte, maxCodeLookups+1))
	if err != nil {
		t.Fatal(err)
	}
	if err := decodeResponseList(tooMany, maxCodeLookups, &got); err == nil || !strings.Contains(err.Error(), "item count") {
		t.Fatalf("decodeResponseList returned %v, want item-count error", err)
	}

	compatible, err := rlp.EncodeToBytes([][]byte{make([]byte, softResponseLimit)})
	if err != nil {
		t.Fatal(err)
	}
	if err := decodeResponseList(compatible, 1, &got); err != nil {
		t.Fatalf("decodeResponseList rejected protocol slack: %v", err)
	}

	tooLarge, err := rlp.EncodeToBytes([][]byte{make([]byte, maxResponseBytes+1)})
	if err != nil {
		t.Fatal(err)
	}
	if err := decodeResponseList(tooLarge, 1, &got); err == nil || !strings.Contains(err.Error(), "encoded size") {
		t.Fatalf("decodeResponseList returned %v, want encoded-size error", err)
	}
}

func TestDecodeTrieNodePathsLimits(t *testing.T) {
	want := []TrieNodePathSet{{{1}, {2}}, {{3}}}
	raw, err := rlp.EncodeToBytes(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeTrieNodePaths(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || len(got[0]) != 2 || len(got[1]) != 1 {
		t.Fatalf("decoded path shape %v, want [2 1]", got)
	}

	tooManySets, err := rlp.EncodeToBytes(make([]TrieNodePathSet, maxTrieNodeLookups+1))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeTrieNodePaths(tooManySets); err == nil || !strings.Contains(err.Error(), "path set count") {
		t.Fatalf("decodeTrieNodePaths returned %v, want path-set error", err)
	}

	validMax := make([]TrieNodePathSet, maxTrieNodeLookups)
	for i := range validMax {
		validMax[i] = TrieNodePathSet{nil, nil}
	}
	raw, err = rlp.EncodeToBytes(validMax)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeTrieNodePaths(raw); err != nil {
		t.Fatalf("decodeTrieNodePaths rejected valid maximum: %v", err)
	}

	tooManyPaths, err := rlp.EncodeToBytes([]TrieNodePathSet{make(TrieNodePathSet, maxPathSegments+1)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeTrieNodePaths(tooManyPaths); err == nil || !strings.Contains(err.Error(), "path count") {
		t.Fatalf("decodeTrieNodePaths returned %v, want path-count error", err)
	}
}
