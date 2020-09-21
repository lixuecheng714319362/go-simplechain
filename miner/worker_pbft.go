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

package miner

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/simplechain-org/go-simplechain/common"
	"github.com/simplechain-org/go-simplechain/consensus/pbft"
	"github.com/simplechain-org/go-simplechain/core"
	"github.com/simplechain-org/go-simplechain/core/state"
	"github.com/simplechain-org/go-simplechain/core/types"
	"github.com/simplechain-org/go-simplechain/log"
)

func (w *worker) Execute(block *types.Block) (*types.Block, error) {
	log.Trace("[debug] Pbft Execute block >>>", "number", block.NumberU64(),
		"pendingHash", block.PendingHash(), "sealHash", w.engine.SealHash(block.Header()), "hash", block.Hash())

	parent := w.chain.GetHeader(block.ParentHash(), block.NumberU64()-1)
	if parent == nil {
		return nil, fmt.Errorf("ancestor block is not exist, parent:%s", block.ParentHash().String())
	}
	statedb, err := state.New(parent.Root, w.chain.StateCache())
	if err != nil {
		//TODO: handle statedb error
		return nil, err
	}

	err = w.eth.TxPool().SenderFromBlocks(types.Blocks{block}) // check tx sender parallelly
	if err != nil {
		return nil, err
	}

	block, env, err := w.executeBlock(block, statedb)
	if err != nil {
		//TODO: handle execute error
		return nil, err
	}

	w.chain.SetExecuteEnvironment(env)

	// update task if seal task exist
	w.pendingMu.Lock()
	if task, exist := w.pendingTasks[w.engine.SealHash(block.Header())]; exist {
		task.receipts = env.Receipts()
		task.state = statedb
	}
	w.pendingMu.Unlock()

	return block, nil
}

func (w *worker) OnTimeout() {
	txNum := int(w.pbftCtx.MaxBlockTxs)
	if w.pbftCtx.MaxBlockTxs > pbft.MaxBlockTxs {
		atomic.StoreUint64(&w.pbftCtx.MaxBlockTxs, pbft.MaxBlockTxs)
	}
	if txNum > 0 && (w.pbftCtx.LastTimeoutTx == 0 || (w.pbftCtx.LastTimeoutTx > txNum && txNum > w.pbftCtx.MaxNoTimeoutTx)) {
		w.pbftCtx.LastTimeoutTx = txNum
	}
	if maxBlockTxs := w.pbftCtx.MaxBlockTxs; maxBlockTxs > 2 {
		atomic.StoreUint64(&w.pbftCtx.MaxBlockTxs, maxBlockTxs/2)
	}
	log.Info("decrease maxBlockCanSeal to half for PBFT timeout",
		"old", w.pbftCtx.MaxBlockTxs*2, "new", w.pbftCtx.MaxBlockTxs, "lastTimeout", w.pbftCtx.LastTimeoutTx)
}

func (w *worker) OnCommit(blockNum uint64, txNum int) {
	//defer func(before uint64) {
	//	log.Error("[debug] pbft context", "before", before, "maxBlockTxs", w.pbftCtx.MaxBlockTxs,
	//		"maxNoTimeoutTx", w.pbftCtx.MaxNoTimeoutTx, "lastTimeoutTx", w.pbftCtx.LastTimeoutTx)
	//}(w.pbftCtx.MaxBlockTxs)

	if w.pbftCtx.MaxBlockTxs >= pbft.MaxBlockTxs {
		atomic.StoreUint64(&w.pbftCtx.MaxBlockTxs, pbft.MaxBlockTxs)
	}
	// old block, ignore
	if blockNum <= w.chain.CurrentBlock().NumberU64() {
		return
	}
	// larger than MaxNoTimeoutTx, increase MaxNoTimeoutTx
	if txNum > 0 && (w.pbftCtx.MaxNoTimeoutTx == 0 || w.pbftCtx.MaxNoTimeoutTx < txNum) {
		w.pbftCtx.MaxNoTimeoutTx = txNum
	}
	if w.pbftCtx.MaxBlockTxs >= pbft.MaxBlockTxs {
		atomic.StoreUint64(&w.pbftCtx.MaxBlockTxs, pbft.MaxBlockTxs)
		return
	}
	if w.pbftCtx.LastTimeoutTx <= w.pbftCtx.MaxNoTimeoutTx {
		w.adjustTimeoutTx()
	}
	if w.pbftCtx.LastTimeoutTx > 0 && w.pbftCtx.MaxBlockTxs >= uint64(w.pbftCtx.LastTimeoutTx) {
		return
	}
	// try increase to 1.5 times
	if maxBlockTx := w.pbftCtx.MaxBlockTxs; maxBlockTx > 2 {
		atomic.AddUint64(&w.pbftCtx.MaxBlockTxs, maxBlockTx/2)
	} else {
		atomic.AddUint64(&w.pbftCtx.MaxBlockTxs, 1)
	}
	// decrease to lastTimeout size
	if w.pbftCtx.LastTimeoutTx > 0 && w.pbftCtx.MaxBlockTxs > uint64(w.pbftCtx.LastTimeoutTx) {
		atomic.StoreUint64(&w.pbftCtx.MaxBlockTxs, uint64(w.pbftCtx.LastTimeoutTx))
	}
	// increase to maxNoTimeout size
	if w.pbftCtx.MaxNoTimeoutTx > 0 && w.pbftCtx.MaxBlockTxs < uint64(w.pbftCtx.MaxNoTimeoutTx) {
		atomic.StoreUint64(&w.pbftCtx.MaxBlockTxs, uint64(w.pbftCtx.MaxNoTimeoutTx))
	}
}

func (w *worker) adjustTimeoutTx() {
	if uint64(w.pbftCtx.LastTimeoutTx) >= pbft.MaxBlockTxs {
		w.pbftCtx.LastTimeoutTx = int(pbft.MaxBlockTxs)
		return
	}
	if uint64(w.pbftCtx.MaxNoTimeoutTx) == pbft.MaxBlockTxs {
		w.pbftCtx.LastTimeoutTx = w.pbftCtx.MaxNoTimeoutTx
		return
	}
	if float64(w.pbftCtx.MaxNoTimeoutTx)*0.1 > 1 {
		w.pbftCtx.LastTimeoutTx = int(float64(w.pbftCtx.MaxNoTimeoutTx) * 1.1)
	} else {
		w.pbftCtx.LastTimeoutTx *= 2
	}
	if uint64(w.pbftCtx.LastTimeoutTx) >= pbft.MaxBlockTxs {
		w.pbftCtx.LastTimeoutTx = int(pbft.MaxBlockTxs)
	}
}

func (w *worker) executeBlock(block *types.Block, statedb *state.StateDB) (*types.Block, *state.ExecutedEnvironment, error) {
	var (
		receipts types.Receipts
		usedGas  = new(uint64)
		header   = block.Header()
		allLogs  []*types.Log
		//
		cfg = *w.chain.GetVMConfig()
	)

	// Iterate over and process the individual transactions
	gp := new(core.GasPool).AddGas(block.GasLimit())
	for i, tx := range block.Transactions() {
		statedb.Prepare(tx.Hash(), block.Hash(), i)
		receipt, err := core.ApplyTransaction(w.chainConfig, w.chain, nil, gp, statedb, header, tx, usedGas, cfg)
		if err != nil {
			return nil, nil, err
		}
		receipts = append(receipts, receipt)
		allLogs = append(allLogs, receipt.Logs...)
	}

	// parallel process transactions TODO: support parallel transactions
	/*var err error
	receipts, allLogs, err = core.ParallelApplyTransactions(w.chainConfig, w.chain, nil, statedb, header, cfg, block.Transactions())
	if err != nil {
		return nil, nil, err
	}*/

	header.GasUsed = *usedGas
	header.Bloom = types.CreateBloom(receipts)
	header.ReceiptHash = types.DeriveSha(receipts)

	if err := w.engine.Finalize(w.chain, header, statedb, block.Transactions(), block.Uncles(), receipts); err != nil {
		return nil, nil, err
	}

	execBlock := block.WithSeal(header) // with seal executed block
	return execBlock, state.NewExecutedEnvironment(block.Hash(), statedb, receipts, allLogs, *usedGas), nil

}

func (w *worker) commitByzantium(interrupt *int32, noempty bool, tstart time.Time) {
	/*defer func(start time.Time) {
		log.Report("commit byzantium seal task", "cost", time.Since(start))
	}(time.Now())*/

	// Fill the block with all available pending transactions.
	maxBlockTxs := atomic.LoadUint64(&w.pbftCtx.MaxBlockTxs)
	pending := w.eth.TxPool().PendingLimit(int(maxBlockTxs))

	//log.Report("commitByzantium -> PendingLimit", "cost", time.Since(start))

	if !noempty && len(pending) == 0 {
		// Create an empty block based on temporary copied state for sealing in advance without waiting block
		// execution finished.
		w._commitByzantium(nil, false, tstart)
	}

	if len(pending) == 0 {
		//TODO-D: don't need update pending state for pending block
		//w.updateSnapshot()
		return
	}

	w.current.txs = pending
	w._commitByzantium(w.fullTaskHook, true, tstart)
}

func (w *worker) _commitByzantium(interval func(), update bool, start time.Time) {
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
				"txs", w.current.tcount, "elapsed", common.PrettyDuration(time.Since(start)), "maxTxsCanSeal", w.pbftCtx.MaxBlockTxs)

		case <-w.exitCh:
			log.Info("Worker has exited")
		}
	}

	if update {
		//TODO-D: don't need update pending state for unexecuted block
		//w.updateSnapshot()
	}
}
