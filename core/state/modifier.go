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

package state

//import (
//	"math/big"
//
//	"github.com/simplechain-org/go-simplechain/common"
//	"github.com/simplechain-org/go-simplechain/core/types"
//	"github.com/simplechain-org/go-simplechain/crypto"
//)
//
//type StateModify struct {
//	// Journal of state modifications. This is the backbone of
//	// Snapshot and RevertToSnapshot.
//	journal        *journal
//	validRevisions []revision
//	nextRevisionId int
//}
//
//func NewStateModify() *StateModify {
//	return &StateModify{
//		journal:        newJournal(),
//		validRevisions: []revision{},
//		nextRevisionId: 0,
//	}
//}
//
//func (s *StateDB) CreateAccountUseDag(addr common.Address, modify *StateModify) {
//	newObj, prev := s.createObjectUseDag(addr, modify)
//	if prev != nil {
//		newObj.setBalance(prev.data.Balance)
//	}
//}
//
//func (s *StateDB) AddLogUseDag(log *types.Log, modify *StateModify) {
//	modify.journal.append(addLogChange{txhash: s.thash})
//
//	log.TxHash = s.thash
//	log.BlockHash = s.bhash
//	log.TxIndex = uint(s.txIndex)
//	log.Index = s.logSize
//	s.logs[s.thash] = append(s.logs[s.thash], log)
//	s.logSize++
//}
//func (s *StateDB) AddPreimageUseDag(hash common.Hash, preimage []byte, modify *StateModify) {
//	if _, ok := s.preimages[hash]; !ok {
//		modify.journal.append(addPreimageChange{hash: hash})
//		pi := make([]byte, len(preimage))
//		copy(pi, preimage)
//		s.preimages[hash] = pi
//	}
//}
//func (s *StateDB) AddRefundUseDag(gas uint64, modify *StateModify) {
//	modify.journal.append(refundChange{prev: s.refund})
//	s.refund += gas
//}
//func (s *StateDB) SubRefundUseDag(gas uint64, modify *StateModify) {
//	modify.journal.append(refundChange{prev: s.refund})
//	if gas > s.refund {
//		panic("Refund counter below zero")
//	}
//	s.refund -= gas
//}
//func (s *StateDB) AddBalanceUseDag(addr common.Address, amount *big.Int, modify *StateModify) {
//	stateObject := s.GetOrNewStateObject(addr)
//	if stateObject != nil {
//		stateObject.AddBalanceUseDag(amount, modify)
//	}
//}
//func (s *StateDB) SetNonceUseDag(addr common.Address, nonce uint64, modify *StateModify) {
//	stateObject := s.GetOrNewStateObject(addr)
//	if stateObject != nil {
//		stateObject.SetNonceUseDag(nonce, modify)
//	}
//}
//func (s *StateDB) SetCodeUseDag(addr common.Address, code []byte, modify *StateModify) {
//	stateObject := s.GetOrNewStateObject(addr)
//	if stateObject != nil {
//		stateObject.SetCodeUseDag(crypto.Keccak256Hash(code), code, modify)
//	}
//}
//func (s *StateDB) SetStateUseDag(addr common.Address, key, value common.Hash, modify *StateModify) {
//	stateObject := s.GetOrNewStateObject(addr)
//	if stateObject != nil {
//		stateObject.SetStateUseDag(s.db, key, value, modify)
//	}
//}
//
//func (s *StateDB) createObjectUseDag(addr common.Address, modify *StateModify) (newobj, prev *stateObject) {
//	prev = s.getDeletedStateObject(addr) // Note, prev might have been deleted, we need that!
//
//	newobj = newObject(s, addr, Account{})
//	newobj.setNonce(0) // sets the object to dirty
//	if prev == nil {
//		modify.journal.append(createObjectChange{account: &addr})
//	} else {
//		modify.journal.append(resetObjectChange{prev: prev})
//	}
//	s.setStateObject(newobj)
//	return newobj, prev
//}
//
//func (s *stateObject) touchUseDag(modify *StateModify) {
//	modify.journal.append(touchChange{
//		account: &s.address,
//	})
//	if s.address == ripemd {
//		// Explicitly put it in the dirty-cache, which is otherwise generated from
//		// flattened journals.
//		modify.journal.dirty(s.address)
//	}
//}
//
//func (s *stateObject) SetStateUseDag(db Database, key, value common.Hash, modify *StateModify) {
//	// If the fake storage is set, put the temporary state update here.
//	if s.fakeStorage != nil {
//		s.fakeStorage[key] = value
//		return
//	}
//	// If the new value is the same as old, don't set
//	prev := s.GetState(db, key)
//	if prev == value {
//		return
//	}
//	// New value is different, update and journal the change
//	modify.journal.append(storageChange{
//		account:  &s.address,
//		key:      key,
//		prevalue: prev,
//	})
//	s.setState(key, value)
//}
//
//func (s *stateObject) AddBalanceUseDag(amount *big.Int, modify *StateModify) {
//	// EIP158: We must check emptiness for the objects such that the account
//	// clearing (0,0,0 objects) can take effect.
//	if amount.Sign() == 0 {
//		if s.empty() {
//			s.touchUseDag(modify)
//		}
//
//		return
//	}
//	s.SetBalanceUseDag(new(big.Int).Add(s.Balance(), amount), modify)
//}
//
//func (s *stateObject) SubBalanceUseDag(amount *big.Int, modify *StateModify) {
//	if amount.Sign() == 0 {
//		return
//	}
//	s.SetBalanceUseDag(new(big.Int).Sub(s.Balance(), amount), modify)
//}
//
//func (s *stateObject) SetBalanceUseDag(amount *big.Int, modify *StateModify) {
//	modify.journal.append(balanceChange{
//		account: &s.address,
//		prev:    new(big.Int).Set(s.data.Balance),
//	})
//	s.setBalance(amount)
//}
//
//func (s *stateObject) SetCodeUseDag(codeHash common.Hash, code []byte, modify *StateModify) {
//	prevcode := s.Code(s.db.db)
//	modify.journal.append(codeChange{
//		account:  &s.address,
//		prevhash: s.CodeHash(),
//		prevcode: prevcode,
//	})
//	s.setCode(codeHash, code)
//}
//
//func (s *stateObject) SetNonceUseDag(nonce uint64, modify *StateModify) {
//	modify.journal.append(nonceChange{
//		account: &s.address,
//		prev:    s.data.Nonce,
//	})
//	s.setNonce(nonce)
//}
