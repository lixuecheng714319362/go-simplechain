// Copyright 2020 The go-simplechain Authors
// This file is part of the go-simplechain library.
//
// The go-simplechain library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-simplechain library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-simplechain library. If not, see <http://www.gnu.org/licenses/>.
//+build sub

package core

import (
	"math/big"
	"testing"

	"github.com/simplechain-org/go-simplechain/core/types"
	"github.com/simplechain-org/go-simplechain/crypto"
	"github.com/stretchr/testify/assert"
)

func TestTxChecker(t *testing.T) {
	txs := getTransactions(2)
	check := NewTxChecker()

	assert.True(t, check.OK(txs[0], false))

	check.InsertCache(txs[0])
	assert.False(t, check.OK(txs[0], false))

	assert.True(t, check.OK(txs[1], false))
	assert.True(t, check.OK(txs[1], true))
	assert.False(t, check.OK(txs[1], false))

	hash := txs[0].Hash()
	check.DeleteCache(hash)
	assert.True(t, check.OK(txs[0], false))

	check.DeleteCaches(txs)
	assert.True(t, check.OK(txs[1], false))
}

func getTransactions(count int, payload ...byte) types.Transactions {
	txs := make(types.Transactions, 0, count)
	for i := 0; i < count; i++ {
		key, _ := crypto.GenerateKey()
		addr := crypto.PubkeyToAddress(key.PublicKey)
		signer := types.NewEIP155Signer(big.NewInt(18))
		tx, _ := types.SignTx(types.NewTransaction(uint64(i), addr, big.NewInt(int64(i)), uint64(i), big.NewInt(int64(i)), payload), signer, key)
		txs = append(txs, tx)
	}
	return txs
}