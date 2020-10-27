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

package miner

import (
	"fmt"
	"github.com/simplechain-org/go-simplechain/core/state"
	"time"

	"github.com/simplechain-org/go-simplechain/common"
	"github.com/simplechain-org/go-simplechain/core/types"
	"github.com/simplechain-org/go-simplechain/log"
)

func (w *worker) commitSolo(interrupt *int32, noempty bool, tstart time.Time) {
	/*defer func(start time.Time) {
		log.Report("commit byzantium seal task", "cost", time.Since(start))
	}(time.Now())*/

	// Fill the block with all available pending transactions.
	pending := w.eth.TxPool().PendingLimit(w.current.header.Number.Uint64(), 50000, true)

	//log.Report("commitByzantium -> PendingLimit", "cost", time.Since(start))

	if !noempty && len(pending) == 0 {
		// Create an empty block based on temporary copied state for sealing in advance without waiting block
		// execution finished.
		w._commitSolo(nil, false, tstart)
	}

	if len(pending) == 0 {
		//TODO-D: don't need update pending state for pending block
		//w.updateSnapshot()
		return
	}

	w.current.txs = pending
	w._commitSolo(w.fullTaskHook, true, tstart)
}

func (w *worker) _commitSolo(interval func(), update bool, start time.Time) {
	//s := time.Now()
	//defer func(s time.Time) {
	//	log.Report("_commitByzantium", "cost", time.Since(s))
	//}(s)

	block := types.NewBlock(w.current.header, w.current.txs, nil, nil)

	//log.Report("_commitByzantium -> NewBlock -> calcRoot", "cost", time.Since(s))

	if w.isRunning() {
		if interval != nil {
			interval()
		}
		select {
		case w.taskCh <- &task{block: block, createdAt: time.Now()}:
			log.Info("Commit new byzantium work", "number", block.Number(), "sealhash", w.engine.SealHash(block.Header()),
				"elapsed", common.PrettyDuration(time.Since(start)))

		case <-w.exitCh:
			log.Info("Worker has exited")
		}
	}

	if update {
		//TODO-D: don't need update pending state for unexecuted block
		//w.updateSnapshot()
	}
}

func (w *worker) ExecuteSolo(block *types.Block) (*types.Block, error) {
	log.Trace("[debug] Pbft Execute block >>>", "number", block.NumberU64(),
		"pendingHash", block.PendingHash(), "sealHash", w.engine.SealHash(block.Header()), "hash", block.Hash())

	parent := w.chain.GetHeader(block.ParentHash(), block.NumberU64()-1)
	if parent == nil {
		return nil, fmt.Errorf("ancestor block is not exist, parent:%s", block.ParentHash().String())
	}

	statedb, err := state.New(parent.Root, w.chain.StateCache())
	if err != nil {
		return nil, err
	}

	block, env, err := w.executeBlock(block, statedb)
	if err != nil {
		return nil, err
	}

	//w.chain.InsertPendingBlock(env)

	// update task if seal task exist
	w.pendingMu.Lock()
	if task, exist := w.pendingTasks[w.engine.SealHash(block.Header())]; exist {
		task.receipts = env.Receipts()
		task.state = statedb
	}
	w.pendingMu.Unlock()

	return block, nil
}
