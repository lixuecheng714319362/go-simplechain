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
	"github.com/simplechain-org/go-simplechain/common"
	"github.com/simplechain-org/go-simplechain/core/state"
	"github.com/simplechain-org/go-simplechain/core/types"
)

type ExecutedBlock struct {
	block    *types.Block
	statedb  *state.StateDB
	receipts types.Receipts
	logs     []*types.Log
}

func NewExecutedBlock(pendingBlock *types.Block, statedb *state.StateDB, receipts types.Receipts, logs []*types.Log) *ExecutedBlock {
	return &ExecutedBlock{
		block:    pendingBlock,
		statedb:  statedb,
		receipts: receipts,
		logs:     logs,
	}
}

func (e *ExecutedBlock) Block() *types.Block {
	return e.block
}

func (e *ExecutedBlock) BlockHash() common.Hash {
	return e.block.Hash()
}

func (e *ExecutedBlock) Statedb() *state.StateDB {
	return e.statedb
}

func (e *ExecutedBlock) Receipts() types.Receipts {
	return e.receipts
}
func (e *ExecutedBlock) Logs() []*types.Log {
	return e.logs
}

func (e *ExecutedBlock) GasUsed() uint64 {
	return e.block.GasUsed()
}
