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
	"sync"

	"github.com/simplechain-org/go-simplechain/common"
	"github.com/simplechain-org/go-simplechain/core/state"
	"github.com/simplechain-org/go-simplechain/core/types"
	"github.com/simplechain-org/go-simplechain/core/vm"
	"github.com/simplechain-org/go-simplechain/crypto"
	"github.com/simplechain-org/go-simplechain/log"
	"github.com/simplechain-org/go-simplechain/params"
)

var evmPool = sync.Pool{
	New: func() interface{} {
		return new(vm.EVM)
	},
}

// ApplyTransaction attempts to apply a transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
func applyCommonTransaction(config *params.ChainConfig, bc ChainContext, author *common.Address, gp *GasPool, statedb *state.StateDB, header *types.Header, tx *types.Transaction, usedGas *uint64, cfg vm.Config) (*types.Receipt, error) {
	msg, err := tx.AsMessage(types.MakeSigner(config))
	if err != nil {
		return nil, err
	}
	// Create a new context to be used in the EVM environment
	context := NewEVMContext(msg, header, bc, author)
	// Create a new environment which holds all relevant information
	// about the transaction and calling mechanisms.
	//vmenv := vm.NewEVM(context, statedb, config, cfg)
	vmenv := evmPool.Get().(*vm.EVM)
	defer evmPool.Put(vmenv)

	vm.PrepareEVM(context, vmenv, statedb, config, cfg)
	// Apply the transaction to the current state (included in the env)
	_, gas, failed, err := ApplyMessage(vmenv, msg, gp)
	if err != nil {
		return nil, err
	}
	// Update the state with pending changes
	var root []byte

	statedb.Finalise(true)

	*usedGas += gas

	// Create a new receipt for the transaction, storing the intermediate root and gas used by the tx
	// based on the eip phase, we're passing whether the root touch-delete accounts.
	receipt := types.NewReceipt(root, failed, *usedGas)
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = gas
	// if the transaction created a contract, store the creation address in the receipt.
	if msg.To() == nil {
		receipt.ContractAddress = crypto.CreateAddress(vmenv.Context.Origin, tx.Nonce())
	}
	// Set the receipt logs and create a bloom for filtering
	receipt.Logs = statedb.GetLogs(tx.Hash())
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})
	receipt.BlockHash = statedb.BlockHash()
	receipt.BlockNumber = header.Number
	receipt.TransactionIndex = uint(statedb.TxIndex())

	return receipt, err
}

func applyTransactionWithErr(config *params.ChainConfig, bc ChainContext, author *common.Address, gp *GasPool,
	statedb *state.StateDB, header *types.Header, tx *types.Transaction, usedGas *uint64, cfg vm.Config) (*types.Receipt, error) {
	receipt, err := applyCommonTransaction(config, bc, author, gp, statedb, header, tx, usedGas, cfg)

	switch err {
	// return a failed receipt
	case
		// insufficient failure
		ErrInsufficientBalanceForGas,
		ErrGasLimitReached,
		// vm failure
		vm.ErrOutOfGas,
		vm.ErrCodeStoreOutOfGas,
		vm.ErrDepth,
		vm.ErrTraceLimitReached,
		vm.ErrInsufficientBalance,
		vm.ErrContractAddressCollision,
		vm.ErrNoCompatibleInterpreter:

		//TODO(yc): pay gas for failed tx
		gasUsed := header.GasLimit - gp.Gas()
		log.Trace("Caught transaction process error", "hash", tx.Hash(), "gas", gasUsed, "err", err)

		receipt := types.NewReceipt(nil, true, *usedGas)
		receipt.TxHash = tx.Hash()
		// Set the receipt logs and create a bloom for filtering
		receipt.Bloom = types.CreateBloom(types.Receipts{receipt})
		receipt.BlockHash = statedb.BlockHash()
		receipt.BlockNumber = header.Number
		receipt.TransactionIndex = uint(statedb.TxIndex())

		return receipt, nil

	default:
		return receipt, err
	}
}

/*
 * TESTING
 */
func applyEmptyTransaction(tx *types.Transaction) (*types.Receipt, error) {
	var root []byte
	receipt := types.NewReceipt(root, false, 0)
	receipt.TxHash = tx.Hash()
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})
	return receipt, nil
}

/*
 * TESTING
 */
func applyAccountTransaction(config *params.ChainConfig, gp *GasPool, statedb *state.StateDB, header *types.Header, tx *types.Transaction, usedGas *uint64) (*types.Receipt, error) {
	msg, err := tx.AsMessage(types.MakeSigner(config))
	if err != nil {
		return nil, err
	}
	sender, to := msg.From(), msg.To()
	statedb.SetNonce(sender, statedb.GetNonce(sender)+1)
	if to != nil {
		balance := tx.Value()
		statedb.AddBalance(*to, balance)
		statedb.SubBalance(sender, balance)
	}
	*usedGas += params.TxGas
	statedb.Finalise(true)
	return applyEmptyTransaction(tx)
}

func ApplyTransaction(config *params.ChainConfig, bc ChainContext, author *common.Address, gp *GasPool, statedb *state.StateDB, header *types.Header, tx *types.Transaction, usedGas *uint64, cfg vm.Config) (*types.Receipt, error) {
	to := tx.To()
	if to != nil && *to == vm.VoidAddress {
		return applyEmptyTransaction(tx) // used for testing
	}
	//return applyAccountTransaction(config, gp, statedb, header, tx, usedGas) // used for testing
	//return applyCommonTransaction(config, bc, author, gp, statedb, header, tx, usedGas, cfg)
	return applyTransactionWithErr(config, bc, author, gp, statedb, header, tx, usedGas, cfg)
}
