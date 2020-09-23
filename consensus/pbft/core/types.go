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

package core

import (
	"bytes"
	"fmt"
	"github.com/simplechain-org/go-simplechain/consensus/pbft"
	"io"
	"sync/atomic"

	"github.com/simplechain-org/go-simplechain/common"
	"github.com/simplechain-org/go-simplechain/rlp"
)

type Engine interface {
	Start() error
	Stop() error

	IsProposer() bool

	// verify if a hash is the same as the proposed block in the current pending request
	//
	// this is useful when the engine is currently the proposer
	//
	// pending request is populated right at the preprepare stage so this would give us the earliest verification
	// to avoid any race condition of coming propagated blocks
	IsCurrentProposal(proposalHash common.Hash) bool
}

type State uint64

const (
	StateAcceptRequest State = iota
	StatePreprepared
	StatePrepared
	StateCommitted
)

func (s State) String() string {
	if s == StateAcceptRequest {
		return "Accept request"
	} else if s == StatePreprepared {
		return "Preprepared"
	} else if s == StatePrepared {
		return "Prepared"
	} else if s == StateCommitted {
		return "Committed"
	} else {
		return "Unknown"
	}
}

// Cmp compares s and y and returns:
//   -1 if s is the previous state of y
//    0 if s and y are the same state
//   +1 if s is the next state of y
func (s State) Cmp(y State) int {
	if uint64(s) < uint64(y) {
		return -1
	}
	if uint64(s) > uint64(y) {
		return 1
	}
	return 0
}

type MsgCode uint8

func (code MsgCode) String() string {
	switch code {
	case msgPreprepare:
		return "Preprepare"
	case msgPrepare:
		return "Prepare"
	case msgCommit:
		return "Commit"
	case msgRoundChange:
		return "RoundChange"
	case msgLightPreprepare:
		return "LightPreprepare"
	case msgGetMissedTxs:
		return "GetMissedTxs"
	case msgMissedTxs:
		return "MissedTxs"
	default:
		return "unknown"
	}
}

const (
	msgPreprepare  = MsgCode(0x00)
	msgPrepare     = MsgCode(0x01)
	msgCommit      = MsgCode(0x02)
	msgRoundChange = MsgCode(0x03)

	msgLightPreprepare = MsgCode(0x10)
	msgGetMissedTxs    = MsgCode(0x11)
	msgMissedTxs       = MsgCode(0x12)
)

func isLightProposalMsg(code MsgCode) bool {
	return code >= msgLightPreprepare && code <= msgMissedTxs
}

type message struct {
	Code          MsgCode
	Msg           []byte
	Address       common.Address
	Signature     []byte
	CommittedSeal []byte

	ForwardNodes []common.Address
	hash         atomic.Value
}

func (m *message) Hash() common.Hash {
	if hash := m.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v := pbft.RLPHash([]interface{}{
		m.Code,
		m.Msg,
		m.Address,
		m.Signature,
		m.CommittedSeal,
	})
	m.hash.Store(v)
	return v
}

// ==============================================
//
// define the functions that needs to be provided for rlp Encoder/Decoder.

// EncodeRLP serializes m into the Ethereum RLP format.
func (m *message) EncodeRLP(w io.Writer) error {
	//return rlp.Encode(w, []interface{}{m.Code, m.Msg, m.Address, m.Signature, m.CommittedSeal})
	return rlp.Encode(w, []interface{}{m.Code, m.Msg, m.Address, m.Signature, m.CommittedSeal, m.ForwardNodes})
}

// DecodeRLP implements rlp.Decoder, and load the consensus fields from a RLP stream.
func (m *message) DecodeRLP(s *rlp.Stream) error {
	var msg struct {
		Code          MsgCode
		Msg           []byte
		Address       common.Address
		Signature     []byte
		CommittedSeal []byte

		ForwardNodes []common.Address
	}

	if err := s.Decode(&msg); err != nil {
		return err
	}
	m.Code, m.Msg, m.Address, m.Signature, m.CommittedSeal = msg.Code, msg.Msg, msg.Address, msg.Signature, msg.CommittedSeal
	m.ForwardNodes = msg.ForwardNodes
	return nil
}

// ==============================================
//
// define the functions that needs to be provided for core.

func (m *message) FromPayload(b []byte, validateFn func([]byte, []byte) (common.Address, error)) error {
	// Decode message
	err := rlp.DecodeBytes(b, &m)
	if err != nil {
		return err
	}

	// Validate message (on a message without Signature)
	if validateFn != nil {
		var payload []byte
		payload, err = m.PayloadNoSig()
		if err != nil {
			return err
		}

		signerAdd, err := validateFn(payload, m.Signature)
		if err != nil {
			return err
		}
		if !bytes.Equal(signerAdd.Bytes(), m.Address.Bytes()) {
			return errInvalidSigner
		}
	}
	return nil
}

func (m *message) Payload() ([]byte, error) {
	return rlp.EncodeToBytes(m)
}

func (m *message) PayloadNoSig() ([]byte, error) {
	return rlp.EncodeToBytes(&message{
		Code:          m.Code,
		Msg:           m.Msg,
		Address:       m.Address,
		Signature:     []byte{},
		CommittedSeal: m.CommittedSeal,
	})
}

func (m *message) Decode(val interface{}) error {
	return rlp.DecodeBytes(m.Msg, val)
}

func (m *message) String() string {
	if m.ForwardNodes != nil {
		return fmt.Sprintf("{Code: %v, Address: %v, Forwards: [ %s]}", m.Code, m.Address.String(), forwards(m.ForwardNodes).String())
	}
	return fmt.Sprintf("{Code: %v, Address: %v}", m.Code, m.Address.String())
}

type forwards []common.Address

func (s forwards) String() string {
	var buf bytes.Buffer
	for _, addr := range s {
		buf.WriteString(addr.String())
		buf.WriteString(" ")
	}
	return buf.String()
}

// ==============================================
//
// helper functions

func Encode(val interface{}) ([]byte, error) {
	return rlp.EncodeToBytes(val)
}
