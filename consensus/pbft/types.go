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

package pbft

import (
	"fmt"
	"io"
	"math/big"

	"github.com/simplechain-org/go-simplechain/common"
	"github.com/simplechain-org/go-simplechain/core/types"
	"github.com/simplechain-org/go-simplechain/rlp"
)

// Proposal supports retrieving height and serialized block to be used during Istanbul consensus.
type Proposal interface {
	// Number retrieves the sequence number of this proposal.
	Number() *big.Int

	PendingHash() common.Hash // unexecuted block hash

	EncodeRLP(w io.Writer) error

	DecodeRLP(s *rlp.Stream) error

	String() string

	FetchMissedTxs(misses []types.MissedTx) (types.Transactions, error)
}

// Conclusion means executed Proposal
type Conclusion interface {
	Proposal
	// Hash retrieves the hash of this proposal.
	Hash() common.Hash
}

// LightProposal is a Proposal without tx body
type LightProposal interface {
	Proposal

	TxDigests() []common.Hash

	Completed() bool // return whether the proposal is completed (no missed txs)

	FillMissedTxs(txs types.Transactions) error
}

type Request struct {
	Proposal Proposal // always common proposal (block with all txs)
}

func (r *Request) TerminalString() string {
	if r == nil {
		return "nil"
	}
	return fmt.Sprintf("hash: %s, number: %d", r.Proposal.PendingHash().TerminalString(), r.Proposal.Number())
}

// View includes a round number and a sequence number.
// Sequence is the block number we'd like to commit.
// Each round has a number and is composed by 3 steps: preprepare, prepare and commit.
//
// If the given block is not accepted by validators, a round change will occur
// and the validators start a new round with round+1.
type View struct {
	Round    *big.Int
	Sequence *big.Int
}

// EncodeRLP serializes b into the Ethereum RLP format.
func (v *View) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{v.Round, v.Sequence})
}

// DecodeRLP implements rlp.Decoder, and load the consensus fields from a RLP stream.
func (v *View) DecodeRLP(s *rlp.Stream) error {
	var view struct {
		Round    *big.Int
		Sequence *big.Int
	}

	if err := s.Decode(&view); err != nil {
		return err
	}
	v.Round, v.Sequence = view.Round, view.Sequence
	return nil
}

func (v *View) String() string {
	return fmt.Sprintf("{Round: %d, Sequence: %d}", v.Round.Uint64(), v.Sequence.Uint64())
}

// Cmp compares v and y and returns:
//   -1 if v <  y
//    0 if v == y
//   +1 if v >  y
func (v *View) Cmp(y *View) int {
	if v.Sequence.Cmp(y.Sequence) != 0 {
		return v.Sequence.Cmp(y.Sequence)
	}
	if v.Round.Cmp(y.Round) != 0 {
		return v.Round.Cmp(y.Round)
	}
	return 0
}

type Preprepare struct {
	View     *View
	Proposal Proposal
}

// EncodeRLP serializes b into the Ethereum RLP format.
func (b *Preprepare) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{b.View, b.Proposal})
}

// DecodeRLP implements rlp.Decoder, and load the consensus fields from a RLP stream.
func (b *Preprepare) DecodeRLP(s *rlp.Stream) error {
	var preprepare struct {
		View     *View
		Proposal *types.Block
	}

	if err := s.Decode(&preprepare); err != nil {
		return err
	}
	b.View, b.Proposal = preprepare.View, preprepare.Proposal

	return nil
}

type LightPreprepare Preprepare

func (b *LightPreprepare) FullPreprepare() *Preprepare {
	return &Preprepare{
		View:     b.View,
		Proposal: Light2Proposal(b.Proposal),
	}
}

// DecodeRLP implements rlp.Decoder, and load the consensus fields from a RLP stream.
// Overwrite Decode method for light block
func (b *LightPreprepare) DecodeRLP(s *rlp.Stream) error {
	var preprepare struct {
		View     *View
		Proposal *types.LightBlock
	}

	if err := s.Decode(&preprepare); err != nil {
		return err
	}
	b.View, b.Proposal = preprepare.View, preprepare.Proposal

	return nil
}

type Subject struct {
	View    *View
	Pending common.Hash
	Digest  common.Hash
}

// EncodeRLP serializes b into the Ethereum RLP format.
func (b *Subject) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{b.View, b.Pending, b.Digest})
}

// DecodeRLP implements rlp.Decoder, and load the consensus fields from a RLP stream.
func (b *Subject) DecodeRLP(s *rlp.Stream) error {
	var subject struct {
		View    *View
		Pending common.Hash
		Digest  common.Hash
	}

	if err := s.Decode(&subject); err != nil {
		return err
	}
	b.View, b.Pending, b.Digest = subject.View, subject.Pending, subject.Digest
	return nil
}

func (b *Subject) String() string {
	return fmt.Sprintf("{View: %v, Pending:%v, Digest: %v}", b.View, b.Pending.String(), b.Digest.String())
}

type MissedReq struct {
	View      *View
	MissedTxs []types.MissedTx
}

func (b *MissedReq) String() string {
	return fmt.Sprintf("{View: %v, Missed:%v}", b.View, len(b.MissedTxs))

}

func (b *MissedReq) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{b.View, b.MissedTxs})
}

// DecodeRLP implements rlp.Decoder, and load the consensus fields from a RLP stream.
func (b *MissedReq) DecodeRLP(s *rlp.Stream) error {
	var subject struct {
		View      *View
		MissedReq []types.MissedTx
	}

	if err := s.Decode(&subject); err != nil {
		return err
	}
	b.View, b.MissedTxs = subject.View, subject.MissedReq
	return nil
}

type MissedResp struct {
	View   *View
	ReqTxs types.Transactions
}

func (b *MissedResp) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{b.View, b.ReqTxs})
}

// DecodeRLP implements rlp.Decoder, and load the consensus fields from a RLP stream.
func (b *MissedResp) DecodeRLP(s *rlp.Stream) error {
	var subject struct {
		View   *View
		ReqTxs types.Transactions
	}

	if err := s.Decode(&subject); err != nil {
		return err
	}
	b.View, b.ReqTxs = subject.View, subject.ReqTxs
	return nil
}

var offsetCodec = &types.OffsetTransactionsCodec{}

func (b *MissedResp) EncodeOffset() ([]byte, error) {
	encTxs, err := offsetCodec.EncodeToBytes(b.ReqTxs)
	if err != nil {
		return nil, err
	}
	encView, err := rlp.EncodeToBytes(b.View)
	if err != nil {
		return nil, err
	}
	return append(encTxs, encView...), nil
}

func (b *MissedResp) DecodeOffset(buf []byte) error {
	var txs = new(types.Transactions)
	n, err := offsetCodec.DecodeBytes(buf, txs)
	if err != nil {
		return err
	}
	if n >= len(buf)-1 {
		return fmt.Errorf("view info missed")
	}
	var view View
	err = rlp.DecodeBytes(buf[n:], &view)
	if err != nil {
		return err
	}
	b.View, b.ReqTxs = &view, *txs
	return nil
}

// Proposal2Light change the common proposal to the light proposal
// return nil if proposal to block failed
func Proposal2Light(proposal Proposal, init bool) LightProposal {
	block, ok := proposal.(*types.Block)
	if !ok {
		return nil
	}
	if !init {
		return &types.LightBlock{Block: *block}
	}
	return types.NewLightBlock(block)
}

// Light2Proposal trans a light proposal to the common proposal
// warning: will panic if proposal is not light
func Light2Proposal(proposal Proposal) Proposal {
	return &proposal.(*types.LightBlock).Block
}
