// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Package consensus implements different Ethereum consensus engines.
package consensus

import (
	"github.com/simplechain-org/go-simplechain/common"
	"github.com/simplechain-org/go-simplechain/core/types"
)

// Constants to match up protocol versions and messages
const (
	Eth63 = 63
	Eth64 = 64
)

var (
	EthProtocol = Protocol{
		Name:     "eth",
		Versions: []uint{Eth64, Eth63},
		Lengths:  map[uint]uint64{Eth64: 17, Eth63: 17},
	}
)

// Protocol defines the protocol of the consensus
type Protocol struct {
	// Official short name of the protocol used during capability negotiation.
	Name string
	// Supported versions of the eth protocol (first is primary).
	Versions []uint
	// Number of implemented message corresponding to different protocol versions.
	Lengths map[uint]uint64
}

// Broadcaster defines the interface to enqueue blocks to fetcher and find peer
type Broadcaster interface {
	// Enqueue add a block into fetcher queue
	Enqueue(id string, block *types.Block)
	// FindPeers retrives peers by addresses
	FindPeers(map[common.Address]bool) map[common.Address]Peer
	// FindRoute
	FindRoute([]common.Address, int, int) map[common.Address]Peer
}

type Sealer interface {
	// Execute block and return executed block
	Execute(block *types.Block) (*types.Block, error)

	OnTimeout()

	OnCommit(blockNum uint64, txNum int)
}

type TxPool interface {
	// Init light block by txpool
	InitLightBlock(pBlock *types.LightBlock) bool
}

// Peer defines the interface to communicate with peer
type Peer interface {
	// Send sends the message to this peer
	Send(msgcode uint64, data interface{}) error
	MarkTransaction(hash common.Hash)
}
