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

package params

import (
	"math/big"
	"reflect"
	"testing"
)

func TestCheckCompatible(t *testing.T) {
	type test struct {
		stored, new *ChainConfig
		head        uint64
		wantErr     *ConfigCompatError
	}
	tests := []test{
		{stored: AllEthashProtocolChanges, new: AllEthashProtocolChanges, head: 0, wantErr: nil},
		{stored: AllEthashProtocolChanges, new: AllEthashProtocolChanges, head: 100, wantErr: nil},
		{
			stored:  &ChainConfig{EIP150Block: big.NewInt(10)},
			new:     &ChainConfig{EIP150Block: big.NewInt(20)},
			head:    9,
			wantErr: nil,
		},
		{
			stored: AllEthashProtocolChanges,
			new:    &ChainConfig{HomesteadBlock: nil},
			head:   3,
			wantErr: &ConfigCompatError{
				What:         "Homestead fork block",
				StoredConfig: big.NewInt(0),
				NewConfig:    nil,
				RewindTo:     0,
			},
		},
		{
			stored: AllEthashProtocolChanges,
			new:    &ChainConfig{HomesteadBlock: big.NewInt(1)},
			head:   3,
			wantErr: &ConfigCompatError{
				What:         "Homestead fork block",
				StoredConfig: big.NewInt(0),
				NewConfig:    big.NewInt(1),
				RewindTo:     0,
			},
		},
		{
			stored: &ChainConfig{HomesteadBlock: big.NewInt(30), EIP150Block: big.NewInt(10)},
			new:    &ChainConfig{HomesteadBlock: big.NewInt(25), EIP150Block: big.NewInt(20)},
			head:   25,
			wantErr: &ConfigCompatError{
				What:         "EIP150 fork block",
				StoredConfig: big.NewInt(10),
				NewConfig:    big.NewInt(20),
				RewindTo:     9,
			},
		},
		{
			stored:  &ChainConfig{ConstantinopleBlock: big.NewInt(30)},
			new:     &ChainConfig{ConstantinopleBlock: big.NewInt(30), PetersburgBlock: big.NewInt(30)},
			head:    40,
			wantErr: nil,
		},
		{
			stored: &ChainConfig{ConstantinopleBlock: big.NewInt(30)},
			new:    &ChainConfig{ConstantinopleBlock: big.NewInt(30), PetersburgBlock: big.NewInt(31)},
			head:   40,
			wantErr: &ConfigCompatError{
				What:         "Petersburg fork block",
				StoredConfig: nil,
				NewConfig:    big.NewInt(31),
				RewindTo:     30,
			},
		},
		{
			stored: &ChainConfig{VinuBLSBlock: big.NewInt(50)},
			new:    &ChainConfig{VinuBLSBlock: big.NewInt(60)},
			head:   55,
			wantErr: &ConfigCompatError{
				What:         "VinuBLS fork block",
				StoredConfig: big.NewInt(50),
				NewConfig:    big.NewInt(60),
				RewindTo:     49,
			},
		},
	}

	for _, test := range tests {
		err := test.stored.CheckCompatible(test.new, test.head)
		if !reflect.DeepEqual(err, test.wantErr) {
			t.Errorf("error mismatch:\nstored: %v\nnew: %v\nhead: %v\nerr: %v\nwant: %v", test.stored, test.new, test.head, err, test.wantErr)
		}
	}
}

func TestRulesCancun(t *testing.T) {
	config := &ChainConfig{
		ChainID:       big.NewInt(1),
		LondonBlock:   big.NewInt(0),
		ShanghaiBlock: big.NewInt(10),
		CancunBlock:   big.NewInt(20),
	}
	if config.Rules(big.NewInt(19)).IsCancun {
		t.Fatal("Cancun active before configured block")
	}
	if !config.Rules(big.NewInt(20)).IsCancun {
		t.Fatal("Cancun inactive at configured block")
	}
}

func TestRulesPrague(t *testing.T) {
	config := &ChainConfig{
		ChainID:       big.NewInt(1),
		LondonBlock:   big.NewInt(0),
		ShanghaiBlock: big.NewInt(10),
		CancunBlock:   big.NewInt(20),
		PragueBlock:   big.NewInt(30),
	}
	if config.Rules(big.NewInt(29)).IsPrague {
		t.Fatal("Prague active before configured block")
	}
	if !config.Rules(big.NewInt(30)).IsPrague {
		t.Fatal("Prague inactive at configured block")
	}
}

func TestRulesVinuBLS(t *testing.T) {
	config := &ChainConfig{
		ChainID:       big.NewInt(1),
		LondonBlock:   big.NewInt(0),
		ShanghaiBlock: big.NewInt(10),
		CancunBlock:   big.NewInt(20),
		PragueBlock:   big.NewInt(30),
		VinuBLSBlock:  big.NewInt(40),
	}
	if config.Rules(big.NewInt(39)).IsVinuBLS {
		t.Fatal("VinuBLS active before configured block")
	}
	if !config.Rules(big.NewInt(40)).IsVinuBLS {
		t.Fatal("VinuBLS inactive at configured block")
	}
}
