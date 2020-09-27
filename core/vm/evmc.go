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

package vm
//
//import (
//	"github.com/simplechain-org/go-simplechain/common"
//	"github.com/simplechain-org/go-simplechain/core/state"
//	"github.com/simplechain-org/go-simplechain/crypto"
//	"github.com/simplechain-org/go-simplechain/params"
//	"math/big"
//	"time"
//)
//
//func (evm *EVM) CallUseDag(caller ContractRef, addr common.Address, input []byte, gas uint64, value *big.Int, modify *state.StateModify) (ret []byte, leftOverGas uint64, err error) {
//	if evm.vmConfig.NoRecursion && evm.depth > 0 {
//		return nil, gas, nil
//	}
//
//	// Fail if we're trying to execute above the call depth limit
//	if evm.depth > int(params.CallCreateDepth) {
//		return nil, gas, ErrDepth
//	}
//	// Fail if we're trying to transfer more than the available balance
//	if !evm.Context.CanTransfer(evm.StateDB, caller.Address(), value) {
//		return nil, gas, ErrInsufficientBalance
//	}
//
//	var (
//		to       = AccountRef(addr)
//		snapshot = evm.StateDB.Snapshot()
//	)
//	if !evm.StateDB.Exist(addr) {
//		precompiles := PrecompiledContractsByzantium
//		if evm.chainRules.IsSingularity {
//			precompiles = PrecompiledContractsIstanbul
//		}
//		if precompiles[addr] == nil && value.Sign() == 0 {
//			// Calling a non existing account, don't do anything, but ping the tracer
//			if evm.vmConfig.Debug && evm.depth == 0 {
//				evm.vmConfig.Tracer.CaptureStart(caller.Address(), addr, false, input, gas, value)
//				evm.vmConfig.Tracer.CaptureEnd(ret, 0, 0, nil)
//			}
//			return nil, gas, nil
//		}
//		evm.StateDB.CreateAccountUseDag(addr, modify)
//	}
//	evm.Transfer(evm.StateDB, caller.Address(), to.Address(), value)
//	// Initialise a new contract and set the code that is to be used by the EVM.
//	// The contract is a scoped environment for this execution context only.
//	contract := NewContract(caller, to, value, gas)
//	contract.SetCallCode(&addr, evm.StateDB.GetCodeHash(addr), evm.StateDB.GetCode(addr))
//
//	// Even if the account has no code, we need to continue because it might be a precompile
//	start := time.Now()
//
//	// Capture the tracer start/end events in debug mode
//	if evm.vmConfig.Debug && evm.depth == 0 {
//		evm.vmConfig.Tracer.CaptureStart(caller.Address(), addr, false, input, gas, value)
//
//		defer func() { // Lazy evaluation of the parameters
//			evm.vmConfig.Tracer.CaptureEnd(ret, gas-contract.Gas, time.Since(start), err)
//		}()
//	}
//	ret, err = run(evm, contract, input, false)
//
//	// When an error was returned by the EVM or when setting the creation code
//	// above we revert to the snapshot and consume any gas remaining. Additionally
//	// when we're in homestead this also counts for code storage gas errors.
//	if err != nil {
//		evm.StateDB.RevertToSnapshot(snapshot)
//		if err != errExecutionReverted {
//			contract.UseGas(contract.Gas)
//		}
//	}
//	return ret, contract.Gas, err
//}
//
//func (evm *EVM) StaticCallUseDag(caller ContractRef, addr common.Address, input []byte, gas uint64, modify *state.StateModify) (ret []byte, leftOverGas uint64, err error) {
//	if evm.vmConfig.NoRecursion && evm.depth > 0 {
//		return nil, gas, nil
//	}
//	// Fail if we're trying to execute above the call depth limit
//	if evm.depth > int(params.CallCreateDepth) {
//		return nil, gas, ErrDepth
//	}
//
//	var (
//		to       = AccountRef(addr)
//		snapshot = evm.StateDB.Snapshot()
//	)
//	// Initialise a new contract and set the code that is to be used by the EVM.
//	// The contract is a scoped environment for this execution context only.
//	contract := NewContract(caller, to, new(big.Int), gas)
//	contract.SetCallCode(&addr, evm.StateDB.GetCodeHash(addr), evm.StateDB.GetCode(addr))
//
//	// We do an AddBalance of zero here, just in order to trigger a touch.
//	// This doesn't matter on Mainnet, where all empties are gone at the time of Byzantium,
//	// but is the correct thing to do and matters on other networks, in tests, and potential
//	// future scenarios
//	evm.StateDB.AddBalanceUseDag(addr, bigZero, modify)
//
//	// When an error was returned by the EVM or when setting the creation code
//	// above we revert to the snapshot and consume any gas remaining. Additionally
//	// when we're in Homestead this also counts for code storage gas errors.
//	ret, err = run(evm, contract, input, true)
//	if err != nil {
//		evm.StateDB.RevertToSnapshot(snapshot)
//		if err != errExecutionReverted {
//			contract.UseGas(contract.Gas)
//		}
//	}
//	return ret, contract.Gas, err
//}
//
//func (evm *EVM) CreateUseDag(caller ContractRef, code []byte, gas uint64, value *big.Int, modify *state.StateModify) (ret []byte, contractAddr common.Address, leftOverGas uint64, err error) {
//	contractAddr = crypto.CreateAddress(caller.Address(), evm.StateDB.GetNonce(caller.Address()))
//	return evm.createUseDag(caller, &codeAndHash{code: code}, gas, value, contractAddr, modify)
//}
//
//func (evm *EVM) createUseDag(caller ContractRef, codeAndHash *codeAndHash, gas uint64, value *big.Int, address common.Address, modify *state.StateModify) ([]byte, common.Address, uint64, error) {
//	// Depth check execution. Fail if we're trying to execute above the
//	// limit.
//	if evm.depth > int(params.CallCreateDepth) {
//		return nil, common.Address{}, gas, ErrDepth
//	}
//	if !evm.CanTransfer(evm.StateDB, caller.Address(), value) {
//		return nil, common.Address{}, gas, ErrInsufficientBalance
//	}
//	nonce := evm.StateDB.GetNonce(caller.Address())
//	evm.StateDB.SetNonceUseDag(caller.Address(), nonce+1, modify)
//
//	// Ensure there's no existing contract already at the designated address
//	contractHash := evm.StateDB.GetCodeHash(address)
//	if evm.StateDB.GetNonce(address) != 0 || (contractHash != (common.Hash{}) && contractHash != emptyCodeHash) {
//		return nil, common.Address{}, 0, ErrContractAddressCollision
//	}
//	// Create a new account on the state
//	snapshot := evm.StateDB.Snapshot()
//	evm.StateDB.CreateAccountUseDag(address, modify)
//	evm.StateDB.SetNonceUseDag(address, 1, modify)
//	evm.Transfer(evm.StateDB, caller.Address(), address, value)
//
//	// Initialise a new contract and set the code that is to be used by the EVM.
//	// The contract is a scoped environment for this execution context only.
//	contract := NewContract(caller, AccountRef(address), value, gas)
//	contract.SetCodeOptionalHash(&address, codeAndHash)
//
//	if evm.vmConfig.NoRecursion && evm.depth > 0 {
//		return nil, address, gas, nil
//	}
//
//	if evm.vmConfig.Debug && evm.depth == 0 {
//		evm.vmConfig.Tracer.CaptureStart(caller.Address(), address, true, codeAndHash.code, gas, value)
//	}
//	start := time.Now()
//
//	ret, err := run(evm, contract, nil, false)
//
//	// check whether the max code size has been exceeded
//	maxCodeSizeExceeded := len(ret) > params.MaxCodeSize
//	// if the contract creation ran successfully and no errors were returned
//	// calculate the gas required to store the code. If the code could not
//	// be stored due to not enough gas set an error and let it be handled
//	// by the error checking condition below.
//	if err == nil && !maxCodeSizeExceeded {
//		createDataGas := uint64(len(ret)) * params.CreateDataGas
//		if contract.UseGas(createDataGas) {
//			evm.StateDB.SetCode(address, ret)
//		} else {
//			err = ErrCodeStoreOutOfGas
//		}
//	}
//
//	// When an error was returned by the EVM or when setting the creation code
//	// above we revert to the snapshot and consume any gas remaining. Additionally
//	// when we're in homestead this also counts for code storage gas errors.
//	if maxCodeSizeExceeded || err != nil {
//		evm.StateDB.RevertToSnapshot(snapshot)
//		if err != errExecutionReverted {
//			contract.UseGas(contract.Gas)
//		}
//	}
//	// Assign err if contract code size exceeds the max while the err is still empty.
//	if maxCodeSizeExceeded && err == nil {
//		err = errMaxCodeSizeExceeded
//	}
//	if evm.vmConfig.Debug && evm.depth == 0 {
//		evm.vmConfig.Tracer.CaptureEnd(ret, gas-contract.Gas, time.Since(start), err)
//	}
//	return ret, address, contract.Gas, err
//}
//
//func (evm *EVM) Create2UseDag(caller ContractRef, code []byte, gas uint64, endowment *big.Int, salt *big.Int, modify *state.StateModify) (ret []byte, contractAddr common.Address, leftOverGas uint64, err error) {
//	codeAndHash := &codeAndHash{code: code}
//	contractAddr = crypto.CreateAddress2(caller.Address(), common.BigToHash(salt), codeAndHash.Hash().Bytes())
//	return evm.createUseDag(caller, codeAndHash, gas, endowment, contractAddr, modify)
//}