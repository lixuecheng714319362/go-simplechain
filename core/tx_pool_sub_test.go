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
	"crypto/ecdsa"
	"math/big"
	"testing"
	"time"

	"github.com/simplechain-org/go-simplechain/common"
	"github.com/simplechain-org/go-simplechain/core/state"
	"github.com/simplechain-org/go-simplechain/core/types"
	"github.com/simplechain-org/go-simplechain/crypto"
	"github.com/simplechain-org/go-simplechain/event"
	"github.com/simplechain-org/go-simplechain/params"
	"github.com/stretchr/testify/assert"
)

// testTxPoolConfig is a transaction pool configuration without stateful disk
// sideeffects used during testing.
var testTxPoolConfig TxPoolConfig

func init() {
	testTxPoolConfig = DefaultTxPoolConfig
	testTxPoolConfig.Journal = ""
}

type testBlockChain struct {
	statedb       *state.StateDB
	gasLimit      uint64
	chainHeadFeed *event.Feed

	hashNum map[common.Hash]uint64
	chain   []*types.Block
	current uint64
}

func newTestBlockChain() *testBlockChain {
	bc := &testBlockChain{
		hashNum:       make(map[common.Hash]uint64, 1),
		chain:         make([]*types.Block, 1),
		chainHeadFeed: new(event.Feed),
		current:       0,
	}
	// make a genesis block[0]
	genesis := types.NewBlock(&types.Header{}, nil, nil, nil)
	bc.hashNum[genesis.Hash()] = 0
	bc.chain[0] = genesis
	return bc
}

func (bc *testBlockChain) GetTransactions(number uint64) types.Transactions {
	if int(number) >= len(bc.chain) {
		return nil
	}
	return bc.chain[number].Transactions()
}

func (bc *testBlockChain) CurrentBlock() *types.Block {
	return bc.chain[bc.current]
}

func (bc *testBlockChain) GetBlock(hash common.Hash, number uint64) *types.Block {
	if int(number) >= len(bc.chain) {
		return nil
	}
	return bc.chain[number]
}

func (bc *testBlockChain) StateAt(common.Hash) (*state.StateDB, error) {
	return bc.statedb, nil
}

func (bc *testBlockChain) SubscribeChainHeadEvent(ch chan<- ChainHeadEvent) event.Subscription {
	return bc.chainHeadFeed.Subscribe(ch)
}

func (bc *testBlockChain) GetBlockNumber(hash common.Hash) uint64 {
	return bc.hashNum[hash]
}

func (bc *testBlockChain) Produce(txs types.Transactions) *types.Block {
	var (
		parentHash common.Hash
		parentTime uint64
	)
	if bc.current > 0 {
		parent := bc.chain[bc.current]
		parentHash = parent.Hash()
		parentTime = parent.Time()
	}

	header := &types.Header{}
	header.Number = new(big.Int).SetUint64(bc.current + 1)
	header.ParentHash = parentHash
	header.Time = parentTime + 1

	block := types.NewBlock(header, txs, nil, nil)

	bc.current++
	bc.hashNum[block.Hash()] = bc.current
	bc.chain = append(bc.chain, block)

	go bc.chainHeadFeed.Send(ChainHeadEvent{Block: block})

	return block
}

func getSignedTx(count int, key *ecdsa.PrivateKey, gas uint64, price *big.Int, nonce, blockLimit uint64) types.Transactions {
	txs := make(types.Transactions, 0, count)
	for i := 0; i < count; i++ {
		addr := crypto.PubkeyToAddress(key.PublicKey)
		signer := types.NewEIP155Signer(params.TestChainConfig.ChainID)
		raw := types.NewTransaction(nonce+uint64(i), addr, big.NewInt(int64(i)), gas, price, nil)
		if blockLimit > 0 {
			raw.SetBlockLimit(blockLimit)
		}
		tx, err := types.SignTx(raw, signer, key)
		if err != nil {
			panic(err)
		}
		txs = append(txs, tx)
	}
	return txs
}

func setupTxPool() (*TxPool, *ecdsa.PrivateKey, *testBlockChain) {
	blockchain := newTestBlockChain()
	key, _ := crypto.GenerateKey()
	pool := NewTxPool(testTxPoolConfig, params.TestChainConfig, blockchain)

	return pool, key, blockchain
}

func TestTxPool_AddLocal(t *testing.T) {
	txpool, key, blockchain := setupTxPool()

	txs := getSignedTx(2, key, 21000, txpool.gasPrice, 0, txpool.chain.CurrentBlock().NumberU64()+1)

	newTx := make(chan NewTxsEvent, 2)
	sub := txpool.SubscribeNewTxsEvent(newTx)
	defer sub.Unsubscribe()

	err := txpool.AddLocal(txs[0])
	assert.NoError(t, err)

	err = txpool.AddLocal(txs[1])
	assert.NoError(t, err)

	for i := 0; i < 2; i++ {
		select {
		case <-newTx:
		case err := <-sub.Err():
			t.Error(err)
		case <-time.After(time.Second):
			t.Error("timeout waiting new tx")
		}
	}

	// check double spend tx in pool
	err = txpool.AddLocal(txs[0])
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "known transaction")

	pendingTxs := txpool.PendingLimit(10)
	assert.Equal(t, 2, pendingTxs.Len())

	// generate a block
	blockchain.Produce(pendingTxs)

	for !txpool.txChecker.OK(txs[0], false) {
		// wait until newHead handled
		time.Sleep(time.Millisecond)
	}

	// check double spend tx in blockchain
	err = txpool.AddLocal(txs[0])
	assert.Equal(t, ErrDuplicated, err)

	// check expired transaction
	tx1 := getSignedTx(1, key, 21000, txpool.gasPrice, 2, txpool.chain.CurrentBlock().NumberU64())[0]
	err = txpool.AddLocal(tx1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired transaction")

	// check underprice transaction
	tx2 := getSignedTx(1, key, 21000, new(big.Int), 3, txpool.chain.CurrentBlock().NumberU64()+1)[0]
	err = txpool.AddLocal(tx2)
	assert.Equal(t, ErrUnderpriced, err)

	// check overflow
	txpool.config.GlobalSlots = 3
	txs2 := getSignedTx(3, key, 21000, txpool.gasPrice, 4, txpool.chain.CurrentBlock().NumberU64()+1)
	for _, tx := range txs2 {
		assert.NoError(t, txpool.AddLocal(tx))
	}

	tx3 := getSignedTx(3, key, 21000, txpool.gasPrice, 7, txpool.chain.CurrentBlock().NumberU64()+1)[0]
	err = txpool.AddLocal(tx3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "txpool is full")
}

func BenchmarkTxPool_AddLocal10000(b *testing.B) { benchmarkAddLocal(b, 10000) }
func BenchmarkTxPool_AddLocal5000(b *testing.B)  { benchmarkAddLocal(b, 5000) }
func BenchmarkTxPool_AddLocal1000(b *testing.B)  { benchmarkAddLocal(b, 1000) }

func benchmarkAddLocal(b *testing.B, size int) {
	txpool, key, _ := setupTxPool()
	txs := getSignedTx(size, key, 21000, txpool.gasPrice, 0, txpool.chain.CurrentBlock().NumberU64()+1)
	txsCh := make(chan NewTxsEvent, size)
	txpool.SubscribeNewTxsEvent(txsCh)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		txpool.clear()
		b.StartTimer()
		for _, tx := range txs {
			txpool.AddLocal(tx)
		}

		for i := 0; i < size; i++ {
			select {
			case <-txsCh:
			case <-time.After(time.Second):
				b.Fatal("timeout")
			}
		}
	}
}
