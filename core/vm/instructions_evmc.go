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

import (
	"github.com/simplechain-org/go-simplechain/common"
	"github.com/simplechain-org/go-simplechain/core/state"
	"github.com/simplechain-org/go-simplechain/core/types"
	"golang.org/x/crypto/sha3"
)

func opSha3UseDag(modify *state.StateModify) executionFunc {
	return func(pc *uint64, interpreter *EVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
		offset, size := stack.pop(), stack.pop()
		data := memory.GetPtr(offset.Int64(), size.Int64())

		if interpreter.hasher == nil {
			interpreter.hasher = sha3.NewLegacyKeccak256().(keccakState)
		} else {
			interpreter.hasher.Reset()
		}
		interpreter.hasher.Write(data)
		interpreter.hasher.Read(interpreter.hasherBuf[:])

		evm := interpreter.evm
		if evm.vmConfig.EnablePreimageRecording {
			evm.StateDB.AddPreimageUseDag(interpreter.hasherBuf, data, modify)
		}

		stack.push(interpreter.intPool.get().SetBytes(interpreter.hasherBuf[:]))

		interpreter.intPool.put(offset, size)
		return nil, nil
	}
}

func opSstoreUseDag(modify *state.StateModify) executionFunc {
	return func(pc *uint64, interpreter *EVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
		loc := common.BigToHash(stack.pop())
		val := stack.pop()
		interpreter.evm.StateDB.SetStateUseDag(contract.Address(), loc, common.BigToHash(val), modify)

		interpreter.intPool.put(val)
		return nil, nil
	}
}

func opCreateUseDag(modify *state.StateModify) executionFunc {
	return func(pc *uint64, interpreter *EVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
		var (
			value        = stack.pop()
			offset, size = stack.pop(), stack.pop()
			input        = memory.GetCopy(offset.Int64(), size.Int64())
			gas          = contract.Gas
		)
		gas -= gas / 64

		contract.UseGas(gas)
		res, addr, returnGas, suberr := interpreter.evm.CreateUseDag(contract, input, gas, value, modify)
		// Push item on the stack based on the returned error. If the ruleset is
		// homestead we must check for CodeStoreOutOfGasError (homestead only
		// rule) and treat as an error, if the ruleset is frontier we must
		// ignore this error and pretend the operation was successful.
		if suberr == ErrCodeStoreOutOfGas {
			stack.push(interpreter.intPool.getZero())
		} else if suberr != nil && suberr != ErrCodeStoreOutOfGas {
			stack.push(interpreter.intPool.getZero())
		} else {
			stack.push(interpreter.intPool.get().SetBytes(addr.Bytes()))
		}
		contract.Gas += returnGas
		interpreter.intPool.put(value, offset, size)

		if suberr == errExecutionReverted {
			return res, nil
		}
		return nil, nil
	}
}

func opCreate2UseDag(modify *state.StateModify) executionFunc {
	return func(pc *uint64, interpreter *EVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
		var (
			endowment    = stack.pop()
			offset, size = stack.pop(), stack.pop()
			salt         = stack.pop()
			input        = memory.GetCopy(offset.Int64(), size.Int64())
			gas          = contract.Gas
		)

		// Apply EIP150
		gas -= gas / 64
		contract.UseGas(gas)
		res, addr, returnGas, suberr := interpreter.evm.Create2UseDag(contract, input, gas, endowment, salt, modify)
		// Push item on the stack based on the returned error.
		if suberr != nil {
			stack.push(interpreter.intPool.getZero())
		} else {
			stack.push(interpreter.intPool.get().SetBytes(addr.Bytes()))
		}
		contract.Gas += returnGas
		interpreter.intPool.put(endowment, offset, size, salt)

		if suberr == errExecutionReverted {
			return res, nil
		}
		return nil, nil
	}
}

func opStaticCallUseDag(modify *state.StateModify) executionFunc {
	return func(pc *uint64, interpreter *EVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
		// Pop gas. The actual gas is in interpreter.evm.callGasTemp.
		interpreter.intPool.put(stack.pop())
		gas := interpreter.evm.callGasTemp
		// Pop other call parameters.
		addr, inOffset, inSize, retOffset, retSize := stack.pop(), stack.pop(), stack.pop(), stack.pop(), stack.pop()
		toAddr := common.BigToAddress(addr)
		// Get arguments from the memory.
		args := memory.GetPtr(inOffset.Int64(), inSize.Int64())

		ret, returnGas, err := interpreter.evm.StaticCallUseDag(contract, toAddr, args, gas, modify)
		if err != nil {
			stack.push(interpreter.intPool.getZero())
		} else {
			stack.push(interpreter.intPool.get().SetUint64(1))
		}
		if err == nil || err == errExecutionReverted {
			memory.Set(retOffset.Uint64(), retSize.Uint64(), ret)
		}
		contract.Gas += returnGas

		interpreter.intPool.put(addr, inOffset, inSize, retOffset, retSize)
		return ret, nil
	}
}

func opSuicideUseDag(modify *state.StateModify) executionFunc {
	return func(pc *uint64, interpreter *EVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
		balance := interpreter.evm.StateDB.GetBalance(contract.Address())
		interpreter.evm.StateDB.AddBalanceUseDag(common.BigToAddress(stack.pop()), balance, modify)

		interpreter.evm.StateDB.Suicide(contract.Address())
		return nil, nil
	}
}

func makeLogUseDag(size int, modify *state.StateModify) executionFunc {
	return func(pc *uint64, interpreter *EVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
		topics := make([]common.Hash, size)
		mStart, mSize := stack.pop(), stack.pop()
		for i := 0; i < size; i++ {
			topics[i] = common.BigToHash(stack.pop())
		}

		d := memory.GetCopy(mStart.Int64(), mSize.Int64())
		interpreter.evm.StateDB.AddLogUseDag(&types.Log{
			Address: contract.Address(),
			Topics:  topics,
			Data:    d,
			// This is a non-consensus field, but assigned here because
			// core/state doesn't know the current block number.
			BlockNumber: interpreter.evm.BlockNumber.Uint64(),
		}, modify)

		interpreter.intPool.put(mStart, mSize)
		return nil, nil
	}
}