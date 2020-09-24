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

package sub

import (
	"time"

	"github.com/simplechain-org/go-simplechain/common/math"
)

func (pm *ProtocolManager) txsyncLoop() {
	for {
		select {
		case s := <-pm.txsyncTaskCh:
			select {
			case s.task <- pm.BroadcastTxs(s.txs):
			case <-pm.quitSync:
			}
		}
	}
}

func (pm *ProtocolManager) syncTransactions(p *peer) {
	// txpool is empty
	size, _ := pm.txpool.Stats()
	if size <= 0 {
		return
	}

	// new peer is not a validator, ignore in router mode,
	if pm.txSyncRouter != nil &&
		!pm.txSyncRouter.IsValidator(pm.peers.Address(p.id)) {
		return
	}

	// no txs need to be synced
	txs := pm.txpool.SyncLimit(size)
	if txs == nil {
		return
	}

	var (
		syncTask func(start int)
		task     chan bool
	)

	task = make(chan bool)
	syncTask = func(start int) {
		// divide txs into tasks
		end := math.IntMin(start+broadcastTxLimit, txs.Len())
		if start >= end {
			return
		}
		select {
		case pm.txsyncTaskCh <- &txsyncTask{txs[start:end], task}:
		case <-pm.quitSync:
			return
		}

		select {
		case s := <-task:
			if !s {
				syncTask(end)
				break
			}
			// if broadcast, wait at least 50ms
			time.AfterFunc(50*time.Millisecond, func() {
				syncTask(end)
			})

		case <-pm.quitSync:
		}
	}
	syncTask(0)
}
