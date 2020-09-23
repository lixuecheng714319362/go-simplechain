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
	"math/big"
	"sync"

	"github.com/simplechain-org/go-simplechain/consensus/pbft"
)

func newRoundChangeSet(valSet pbft.ValidatorSet) *roundChangeSet {
	return &roundChangeSet{
		validatorSet: valSet,
		roundChanges: make(map[uint64]*messageSet),
		mu:           new(sync.Mutex),
	}
}

type roundChangeSet struct {
	validatorSet pbft.ValidatorSet
	roundChanges map[uint64]*messageSet
	mu           *sync.Mutex
}

// Add adds the round and message into round change set
func (rcs *roundChangeSet) Add(r *big.Int, msg *message) (int, error) {
	rcs.mu.Lock()
	defer rcs.mu.Unlock()

	round := r.Uint64()
	if rcs.roundChanges[round] == nil {
		rcs.roundChanges[round] = newMessageSet(rcs.validatorSet)
	}
	err := rcs.roundChanges[round].Add(msg)
	if err != nil {
		return 0, err
	}
	return rcs.roundChanges[round].Size(), nil
}

// Clear deletes the messages with smaller round
func (rcs *roundChangeSet) Clear(round *big.Int) {
	rcs.mu.Lock()
	defer rcs.mu.Unlock()

	for k, rms := range rcs.roundChanges {
		if len(rms.Values()) == 0 || k < round.Uint64() {
			delete(rcs.roundChanges, k)
		}
	}
}

// MaxRound returns the max round which the number of messages is equal or larger than num
func (rcs *roundChangeSet) MaxRound(num int) *big.Int {
	rcs.mu.Lock()
	defer rcs.mu.Unlock()

	var maxRound *big.Int
	for k, rms := range rcs.roundChanges {
		if rms.Size() < num {
			continue
		}
		r := big.NewInt(int64(k))
		if maxRound == nil || maxRound.Cmp(r) < 0 {
			maxRound = r
		}
	}
	return maxRound
}
