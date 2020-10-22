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

package core

import (
	"github.com/simplechain-org/go-simplechain/core/types"
)

func (pool *TxPool) Signer() types.Signer {
	return pool.signer
}

func (pool *TxPool) InitLightBlock(pb *types.LightBlock) bool {
	digests := pb.TxDigests()
	transactions := pb.Transactions()

	for index, hash := range digests {
		if tx := pool.all.Get(hash); tx != nil {
			(*transactions)[index] = tx
		} else {
			pb.MissedTxs = append(pb.MissedTxs, types.MissedTx{Hash: hash, Index: uint32(index)})
		}
	}

	return len(pb.MissedTxs) == 0
}
