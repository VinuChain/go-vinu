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

	tooLarge, err := rlp.EncodeToBytes([][]byte{make([]byte, softResponseLimit+1)})
	if err != nil {
		t.Fatal(err)
	}
	if err := decodeResponseList(tooLarge, 1, &got); err == nil || !strings.Contains(err.Error(), "encoded size") {
		t.Fatalf("decodeResponseList returned %v, want encoded-size error", err)
	}
}
