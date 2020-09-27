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

//TODO(plan): support concurrent execute block
// parallel apply transactions by txDAG

//
//import (
//	"runtime"
//	"sync"
//	"time"
//
//	"github.com/exascience/pargo/parallel"
//	"github.com/simplechain-org/go-simplechain/common"
//	"github.com/simplechain-org/go-simplechain/core/state"
//	"github.com/simplechain-org/go-simplechain/core/types"
//	"github.com/simplechain-org/go-simplechain/core/vm"
//	"github.com/simplechain-org/go-simplechain/crypto"
//	"github.com/simplechain-org/go-simplechain/dag"
//	"github.com/simplechain-org/go-simplechain/log"
//	"github.com/simplechain-org/go-simplechain/params"
//)
//
//func ParallelApplyTransactions(config *params.ChainConfig, bc ChainContext, coinbase *common.Address,
//	statedb *state.StateDB, header *types.Header, cfg vm.Config, txs types.Transactions) (types.Receipts, []*types.Log, error) {
//	dag.Clear()
//	graph := dag.NewGraph(config.ChainID)
//	err := graph.InitGraphWithTxPriceAndNonce(txs, txs.Len())
//	if err != nil {
//		log.Error("use DAG to InitGraphWithTxPriceAndNonce failed", "error", err)
//		return nil, nil, err
//	}
//	var (
//		usedGas       = new(uint64)
//		gp            = new(GasPool).AddGas(header.GasLimit)
//		receipts      = make(types.Receipts, 0, txs.Len()) // TODO: fill receipts by tx index
//		coalescedLogs []*types.Log
//	)
//
//	for {
//		if dag.TopLevel.Len() == 0 {
//			break
//		}
//		execIds, err := dag.WaitPop()
//		//log.Error("@@ execIds @@@", "len", len(execIds))
//		if err != nil {
//			log.Error("use DAG to InitGraphWithTxPriceAndNonce failed", "error", err)
//			return nil, nil, err
//		}
//		if len(execIds) == 0 {
//			log.Info("transaction execute finished!")
//			return nil, nil, nil
//		}
//
//		rs, logs, err := ParallelApplyTransaction(config, bc, gp, statedb, header, coinbase, usedGas, cfg, execIds)
//		if err != nil {
//			log.Error("ParallelApplyTransaction failed", "error", err)
//			return nil, nil, err
//		}
//		receipts = append(receipts, rs...) //TODO
//		coalescedLogs = append(coalescedLogs, logs...)
//	}
//
//	//TODO: handle coalescedLogs event
//
//	return receipts, coalescedLogs, nil
//}
//
//func ParallelApplyTransaction(config *params.ChainConfig, bc ChainContext, gp *GasPool, statedb *state.StateDB,
//	header *types.Header, author *common.Address, usedGas *uint64, cfg vm.Config, execIds []int) (types.Receipts, []*types.Log, error) {
//	var (
//		coalescedLogs = make([]*types.Log, 0)
//		receipts      = make(types.Receipts, 0)
//	)
//
//	evmc := NewConcurrentExecEvm()
//
//	parallel.Range(0, len(execIds), runtime.NumCPU(), func(low, high int) {
//		for i := low; i < high; i++ {
//			tx := dag.V_txs[execIds[i]].Tx
//			start := time.Now()
//			evmc.ApplyTransactionUseDag(config, bc, gp, statedb, header, author, tx, usedGas, cfg)
//			log.Error("ApplyTransactionUseDag ###", "cost", time.Since(start))
//		}
//	})
//
//	for i := 0; i < len(evmc.Tx); i++ {
//		//tx := evmc.Tx[i]
//		err := evmc.Err[i]
//
//		//TODO: check error
//		//from, _ := types.Sender(graph.Singer, tx)
//		switch err {
//		//case core.ErrGasLimitReached:
//		//	// Pop the current out-of-gas transaction without shifting in the next from the account
//		//	log.Error("Gas limit exceeded for current block", "sender", from)
//		//
//		//case core.ErrNonceTooLow:
//		//	// New head notification data race between the transaction pool and miner, shift
//		//	log.Error("Skipping transaction with low nonce", "sender", from, "nonce", tx.Nonce())
//		//
//		//case core.ErrNonceTooHigh:
//		//	// Reorg notification data race between the transaction pool and miner, skip account =
//		//	log.Error("Skipping account with hight nonce", "sender", from, "nonce", tx.Nonce())
//		//
//		//case nil:
//		//	// Everything ok, collect the logs and shift in the next transaction from the same account
//		//	log.Info("execute tx success", "sender", from, "txhash", tx.Hash().String())
//		//	coalescedLogs = append(coalescedLogs, evmc.Receipts[i].Logs...)
//		//	w.current.tcount++
//		//
//		//default:
//		//	// Strange error, discard the transaction and get the next in line (note, the
//		//	// nonce-too-high clause will prevent us from executing in vain).
//		//	log.Error("Transaction failed, account skipped", "hash", tx.Hash(), "err", err)
//		case nil:
//			coalescedLogs = append(coalescedLogs, evmc.Receipts[i].Logs...)
//			receipts = append(receipts, evmc.Receipts[i])
//		}
//		//if err == nil {
//		//	w.current.txs = append(w.current.txs, tx)
//		//	w.current.receipts = append(w.current.receipts, evmc.Receipts[i])
//		//}
//	}
//	return receipts, coalescedLogs, nil
//}
//
//type ConcurrentExecEvm struct {
//	Mu       sync.Mutex
//	Receipts []*types.Receipt
//	Err      []error
//	Tx       []*types.Transaction
//}
//
//func NewConcurrentExecEvm() *ConcurrentExecEvm {
//	return &ConcurrentExecEvm{
//		Mu:       sync.Mutex{},
//		Receipts: make([]*types.Receipt, 0),
//		Err:      make([]error, 0),
//		Tx:       make([]*types.Transaction, 0),
//	}
//}
//
//func (cee *ConcurrentExecEvm) Clear() {
//	cee.Receipts = make([]*types.Receipt, 0)
//	cee.Err = make([]error, 0)
//	cee.Tx = make([]*types.Transaction, 0)
//}
//
//func (cee *ConcurrentExecEvm) ApplyTransactionUseDag(config *params.ChainConfig, bc ChainContext, gp *GasPool,
//	statedb *state.StateDB, header *types.Header, author *common.Address, tx *types.Transaction, usedGas *uint64, cfg vm.Config) {
//
//	cee.Tx = append(cee.Tx, tx)
//	msg, err := tx.AsMessage(types.MakeSigner(config))
//	if err != nil {
//		cee.Mu.Lock()
//		cee.Err = append(cee.Err, err)
//		cee.Receipts = append(cee.Receipts, nil)
//		cee.Mu.Unlock()
//		return
//	}
//
//	// Create a new context to be used in the EVM environment
//	context := NewEVMContext(msg, header, bc, author)
//	// Create a new environment which holds all relevant information
//	// about the transaction and calling mechanisms.
//	vmenv := vm.NewEVM(context, statedb, config, cfg)
//	// Apply the transaction to the current state (included in the env)
//	cee.Mu.Lock()
//	defer cee.Mu.Unlock()
//
//	snap := statedb.Snapshot()
//	modify := state.NewStateModify()
//	_, gas, failed, err := ApplyMessageUseDag(vmenv, msg, gp, modify)
//	if err != nil {
//		cee.Err = append(cee.Err, err)
//		cee.Receipts = append(cee.Receipts, nil)
//		//rollback statedb
//		statedb.RevertToSnapshot(snap)
//		return
//	}
//
//	// Update the state with pending changes
//	var root []byte
//	statedb.Finalise(true)
//	*usedGas += gas
//
//	// Create a new receipt for the transaction, storing the intermediate root and gas used by the tx
//	// based on the eip phase, we're passing whether the root touch-delete accounts.
//	receipt := types.NewReceipt(root, failed, *usedGas)
//	receipt.TxHash = tx.Hash()
//	receipt.GasUsed = gas
//	// if the transaction created a contract, store the creation address in the receipt.
//	if msg.To() == nil {
//		receipt.ContractAddress = crypto.CreateAddress(vmenv.Context.Origin, tx.Nonce())
//	}
//	// Set the receipt logs and create a bloom for filtering
//	receipt.Logs = statedb.GetLogs(tx.Hash())
//	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})
//	receipt.BlockHash = statedb.BlockHash()
//	receipt.BlockNumber = header.Number
//	receipt.TransactionIndex = uint(statedb.TxIndex())
//	cee.Receipts = append(cee.Receipts, receipt)
//	cee.Err = append(cee.Err, nil)
//}
