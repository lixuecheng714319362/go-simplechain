// Copyright 2015 The go-simplechain Authors
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
	"bytes"
	"github.com/simplechain-org/go-simplechain/common"
	"github.com/simplechain-org/go-simplechain/consensus"
	"github.com/simplechain-org/go-simplechain/core/state"
	"github.com/simplechain-org/go-simplechain/core/types"
	"github.com/simplechain-org/go-simplechain/core/vm"
	"github.com/simplechain-org/go-simplechain/crypto"
	"github.com/simplechain-org/go-simplechain/log"
	"github.com/simplechain-org/go-simplechain/params"
	"math/big"
)

// StateProcessor is a basic Processor, which takes care of transitioning
// state from one point to another.
//
// StateProcessor implements Processor.
type StateProcessor struct {
	config *params.ChainConfig // Chain configuration options
	bc     *BlockChain         // Canonical block chain
	engine consensus.Engine    // Consensus engine used for block rewards
}

// NewStateProcessor initialises a new StateProcessor.
func NewStateProcessor(config *params.ChainConfig, bc *BlockChain, engine consensus.Engine) *StateProcessor {
	return &StateProcessor{
		config: config,
		bc:     bc,
		engine: engine,
	}
}

// Process processes the state changes according to the Ethereum rules by running
// the transaction messages using the statedb and applying any rewards to both
// the processor (coinbase) and any included uncles.
//
// Process returns the receipts and logs accumulated during the process and
// returns the amount of gas that was used in the process. If any of the
// transactions failed to execute due to insufficient gas it will return an error.
func (p *StateProcessor) Process(block *types.Block, statedb *state.StateDB, cfg vm.Config, contractAddress common.Address) (types.Receipts, []*types.Log, uint64, error) {
	var (
		receipts types.Receipts
		usedGas  = new(uint64)
		header   = block.Header()
		allLogs  []*types.Log
		gp       = new(GasPool).AddGas(block.GasLimit())
	)

	// Iterate over and process the individual transactions
	for i, tx := range block.Transactions() {
		statedb.Prepare(tx.Hash(), block.Hash(), i)
		receipt, err := ApplyTransaction(p.config, p.bc, nil, gp, statedb, header, tx, usedGas, cfg, contractAddress)
		if err != nil {
			return nil, nil, 0, err
		}
		receipts = append(receipts, receipt)
		allLogs = append(allLogs, receipt.Logs...)
	}
	// Finalize the block, applying any consensus engine specific extras (e.g. block rewards)
	return receipts, allLogs, *usedGas, p.engine.Finalize(p.bc, header, statedb, block.Transactions(), block.Uncles(), receipts)
}

// ApplyTransaction attempts to apply a transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
func ApplyTransaction(config *params.ChainConfig, bc ChainContext, author *common.Address, gp *GasPool, statedb *state.StateDB, header *types.Header, tx *types.Transaction, usedGas *uint64, cfg vm.Config, address common.Address) (*types.Receipt, error) {
	msg, err := tx.AsMessage(types.MakeSigner(config))
	if err != nil {
		return nil, err
	}

	if len(tx.Data()) > 68 && tx.To() != nil && (*tx.To() == address) {
		evmInvoke := NewEvmInvoke(bc, header, statedb, config, cfg)
		if bytes.Equal(tx.Data()[:4], params.FinishFn) {
			// call -> function getMakerTx(bytes32 txId, uint remoteChainId) public view returns(uint)
			paddedCtxId := common.LeftPadBytes(tx.Data()[4+32*3:4+32*4], 32) //CtxId
			remoteChainId := tx.Data()[4+32 : 4+32*2]
			res, err := evmInvoke.CallContract(common.Address{}, tx.To(), params.GetMakerTxFn, paddedCtxId, remoteChainId)
			if err != nil {
				log.Info("Apply makerFinish Transaction failed", "err", err)
				return nil, err
			}
			if new(big.Int).SetBytes(res).Cmp(big.NewInt(0)) == 0 {
				//log.Info("already finish!", "res", new(big.Int).SetBytes(res).Uint64(), "tx", tx.Hash().String())
				return nil, ErrRepetitionCrossTransaction
			} else { //TODO 交易失败一直finish ok
				//log.Info("finish ok!", "res", new(big.Int).SetBytes(res).Uint64(), "tx", tx.Hash().String())
			}

		} else if bytes.Equal(tx.Data()[:4], params.TakerFn) {
			log.Error("[debug] taker Tx", "id", tx.Hash().String())
			// call -> function getTakerTx(bytes32 txId, uint remoteChainId) public view returns(uint)
			paddedCtxId := common.LeftPadBytes(tx.Data()[4+32*4:4+32*5], 32) //CtxId
			remoteChainId := tx.Data()[4+32 : 4+32*2]
			res, err := evmInvoke.CallContract(common.Address{}, tx.To(), params.GetTakerTxFn, paddedCtxId, remoteChainId)
			if err != nil {
				log.Info("Apply taker Transaction failed", "err", err)
				return nil, err
			}
			if new(big.Int).SetBytes(res).Cmp(big.NewInt(0)) == 0 {
				log.Error("[debug] taker Tx check ok", "id", tx.Hash().String())
				//log.Info("take ok!", "res", new(big.Int).SetBytes(res).Uint64(), "tx", tx.Hash().String())
			} else {
				log.Error("[debug] taker Tx ErrRepetitionCrossTransaction", "id", tx.Hash().String())
				//log.Info("already take!", "res", new(big.Int).SetBytes(res).Uint64(), "tx", tx.Hash().String())
				return nil, ErrRepetitionCrossTransaction
			}
		}
	}
	// Create a new context to be used in the EVM environment
	context := NewEVMContext(msg, header, bc, author)
	// Create a new environment which holds all relevant information
	// about the transaction and calling mechanisms.
	vmenv := vm.NewEVM(context, statedb, config, cfg)
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
