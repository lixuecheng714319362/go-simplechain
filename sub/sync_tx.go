package sub

import (
	"runtime"
	"time"

	"github.com/simplechain-org/go-simplechain/consensus"
	"github.com/simplechain-org/go-simplechain/core"
	"github.com/simplechain-org/go-simplechain/core/types"
	"github.com/simplechain-org/go-simplechain/log"

	"github.com/exascience/pargo/parallel"
)

func (pm *ProtocolManager) handleTxs(legacy bool) {
	// broadcast transactions
	pm.txsCh = make(chan core.NewTxsEvent, txChanSize)
	pm.txsSub = pm.txpool.SubscribeNewTxsEvent(pm.txsCh)

	if legacy {
		go pm.txBroadcastLoopLegacy()

	} else {
		pm.txSyncPeriod = defaultTxSyncPeriod
		pm.txSyncTimer = time.NewTimer(pm.txSyncPeriod)

		go pm.txCollectLoop()
		go pm.txBroadcastLoop()
	}
}

func (pm *ProtocolManager) handleRemoteTxsByRouter(peer *peer, txr *TransactionsWithRoute) {
	txRouter := pm.txSyncRouter
	if num := pm.blockchain.CurrentBlock().NumberU64(); num != txRouter.BlockNumber() {
		pre, ok := pm.engine.(consensus.Predictable)
		if !ok {
			log.Warn("handle route tx without byzantine consensus")
			return
		}
		validators, myIndex := pre.CurrentValidators()
		txRouter.Reset(num, validators, myIndex)
	}

	routeIndex := txr.RouteIndex
	selectedNode := pm.txSyncRouter.SelectNodes(pm.peers.PeerWithAddresses(), int(routeIndex), false)
	// forward the received txs
	for _, p := range selectedNode {
		if p == peer {
			continue
		}
		p.AsyncSendTransactionsByRouter(txr)
	}
}

func (pm *ProtocolManager) addRemoteTxsByRouter2TxPool(peer *peer, txr *TransactionsWithRoute) {
	//start := time.Now()

	// parallel check sender
	errs := make([]error, txr.Txs.Len())

	parallel.Range(0, txr.Txs.Len(), runtime.GOMAXPROCS(0), func(low, high int) {
		for i := low; i < high; i++ {
			txr.Txs[i].SetSynced(true)
			_, errs[i] = types.Sender(pm.txpool.Signer(), txr.Txs[i])
		}
	})

	//senderCost := time.Since(start)
	//addTime := time.Now()

	// handle errors and add to txpool
	for i, err := range errs {
		if err == nil {
			err = pm.txpool.AddRemoteSync(txr.Txs[i])
		}
		if err != nil {
			log.Trace("Failed adding remote tx by router", "hash", txr.Txs[i].Hash(), "err", err, "peer", peer)
		}
	}
	//log.Trace("[report] add remote txs received by router", "size", txr.Txs.Len(),
	//	"startTime", start, "senderCost", senderCost, "addCost", time.Since(addTime))
}

// BroadcastTxs will propagate a batch of transactions to all peers which are not known to
// already have the given transaction.
func (pm *ProtocolManager) BroadcastTxs(txs types.Transactions) bool {
	// FIXME include this again: peers = peers[:int(math.Sqrt(float64(len(peers))))]
	txset, routeIndex := pm.CalcTxsWithPeerSet(txs)
	for peer, txs := range txset {
		if routeIndex < 0 {
			peer.AsyncSendTransactions(txs)
		} else {
			peer.AsyncSendTransactionsByRouter(&TransactionsWithRoute{Txs: txs, RouteIndex: uint32(routeIndex)})
		}
	}
	return len(txset) > 0
}

func (pm *ProtocolManager) CalcTxsWithPeerSet(txs types.Transactions) (map[*peer]types.Transactions, int) {
	switch pm.engine.(type) {
	case consensus.Byzantine:
		return pm.calcTxsWithPeerSetByRouter(txs)
	default:
		return pm.calcTxsWithPeerSetStandard(txs), -1
	}
}

func (pm *ProtocolManager) calcTxsWithPeerSetStandard(txs types.Transactions) map[*peer]types.Transactions {
	var txset = make(map[*peer]types.Transactions)

	// Broadcast transactions to a batch of peers not knowing about it
	for _, tx := range txs {
		peers := pm.peers.PeersWithoutTx(tx.Hash())
		for _, peer := range peers {
			txset[peer] = append(txset[peer], tx)
		}
		tx.SetSynced(true) // Mark tx synced
		log.Trace("Broadcast transaction", "hash", tx.Hash(), "recipients", len(peers))
	}

	return txset
}

func (pm *ProtocolManager) calcTxsWithPeerSetByRouter(txs types.Transactions) (map[*peer]types.Transactions, int) {
	txset := make(map[*peer]types.Transactions)
	txRouter := pm.txSyncRouter

	if current := pm.blockchain.CurrentBlock().NumberU64(); current != txRouter.BlockNumber() {
		validators, index := pm.engine.(consensus.Predictable).CurrentValidators()
		txRouter.Reset(current, validators, index)
	}

	routeIndex := txRouter.MyIndex()
	// no validators exist
	if routeIndex < 0 {
		return nil, -1
	}
	selectedNodes := txRouter.SelectNodes(pm.peers.PeerWithAddresses(), routeIndex, true)

	for _, tx := range txs {
		for _, peer := range selectedNodes {
			if !peer.HasTransaction(tx.Hash()) {
				txset[peer] = append(txset[peer], tx)
			}
		}
	}
	return txset, routeIndex
}

// txCollectLoop collect txs from the txsCh
func (pm *ProtocolManager) txCollectLoop() {
	for {
		select {
		case ev := <-pm.txsCh:
			pm.newTxLock.Lock()
			for _, tx := range ev.Txs {
				if !tx.IsSynced() {
					pm.newTransactions = append(pm.newTransactions, ev.Txs...)
				}
			}
			if pm.newTransactions.Len() >= broadcastTxLimit {
				pm.BroadcastTxs(pm.newTransactions)
				pm.newTransactions = pm.newTransactions[:0]
			}
			pm.newTxLock.Unlock()

		case <-pm.txsSub.Err():
			return
		}
	}
}

func (pm *ProtocolManager) txBroadcastLoopLegacy() {
	for {
		select {
		case ev := <-pm.txsCh:
			pm.dumpTxs(ev.Txs)
			pm.BroadcastTxs(ev.Txs)

		// Err() channel will be closed when unsubscribing.
		case <-pm.txsSub.Err():
			return
		}
	}
}

func (pm *ProtocolManager) txBroadcastLoop() {
	var total int

	for {
		select {
		case <-pm.txSyncTimer.C:
			var txs types.Transactions
			pm.newTxLock.Lock()
			if pm.newTransactions.Len() > broadcastTxLimit {
				txs = pm.newTransactions[:broadcastTxLimit]
				pm.newTransactions = pm.newTransactions[broadcastTxLimit:]
			} else {
				txs = append(pm.newTransactions, pm.txpool.SyncLimit(broadcastTxLimit-pm.newTransactions.Len())...)
				pm.newTransactions = pm.newTransactions[:0]
			}
			pm.newTxLock.Unlock()

			if l := txs.Len(); l > 0 {
				total += l
				log.Trace("dump transactions", "total", total, "count", l)
				pm.BroadcastTxs(txs)
			}

			pm.txSyncTimer.Reset(pm.txSyncPeriod)

		case <-pm.quitSync:
			return
		}
	}
}

// dump txs
func (pm *ProtocolManager) dumpTxs(txs types.Transactions) {
	select {
	case ev := <-pm.txsCh:
		txs = append(txs, ev.Txs...)
		if len(txs) >= broadcastTxLimit {
			return
		}
		pm.dumpTxs(txs)

	default:
		log.Trace("dump transactions from txsCh", "count", txs.Len())
	}
}
