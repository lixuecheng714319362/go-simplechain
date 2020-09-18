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

package types

import (
	"math/big"
	"testing"

	"github.com/simplechain-org/go-simplechain/common"
)

var (
	signer = NewEIP155Signer(big.NewInt(110))
)

func BenchmarkSyncListSha10000(b *testing.B) { benchmarkDeriveSha(b, DeriveListSha, 10000) }
func BenchmarkSyncListSha5000(b *testing.B)  { benchmarkDeriveSha(b, DeriveListSha, 5000) }
func BenchmarkSyncListSha1000(b *testing.B)  { benchmarkDeriveSha(b, DeriveListSha, 1000) }

func BenchmarkLegacySha10000(b *testing.B) { benchmarkDeriveSha(b, DeriveLegacySha, 10000) }
func BenchmarkLegacySha5000(b *testing.B)  { benchmarkDeriveSha(b, DeriveLegacySha, 5000) }
func BenchmarkLegacySha1000(b *testing.B)  { benchmarkDeriveSha(b, DeriveLegacySha, 1000) }

func BenchmarkParallelSha10000(b *testing.B) { benchmarkDeriveSha(b, DeriveListShaParallel, 10000) }
func BenchmarkParallelSha5000(b *testing.B)  { benchmarkDeriveSha(b, DeriveListShaParallel, 5000) }
func BenchmarkParallelSha1000(b *testing.B)  { benchmarkDeriveSha(b, DeriveListShaParallel, 1000) }

func benchmarkDeriveSha(b *testing.B, sha func(DerivableList) common.Hash, size int) {
	txs := getTransactions(size)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sha(txs)
	}
}
