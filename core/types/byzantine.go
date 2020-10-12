// Copyright 2014 The go-simplechain Authors
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

package types

import (
	"errors"
	"io"

	"github.com/simplechain-org/go-simplechain/common"
	"github.com/simplechain-org/go-simplechain/rlp"
)

var (
	// to identify whether the block is from Byzantine consensus engine
	// IstanbulDigest represents a hash of "Istanbul practical byzantine fault tolerance"
	IstanbulDigest = common.HexToHash("0x63746963616c2062797a616e74696e65206661756c7420746f6c6572616e6365")
	// IstanbulDigest represents a hash of "Parallel byzantine fault tolerance"
	PbftDigest = common.HexToHash("0x72616c6c656c2062797a616e74696e65206661756c7420746f6c6572616e6365")

	ByzantineExtraVanity = 32 // Fixed number of extra-data bytes reserved for validator vanity
	ByzantineExtraSeal   = 65 // Fixed number of extra-data bytes reserved for validator seal

	// ErrInvalidByzantineHeaderExtra is returned if the length of extra-data is less than 32 bytes
	ErrInvalidByzantineHeaderExtra = errors.New("invalid byzantine header extra-data")
	// ErrFailToFetchLightMissedTxs is returned if fetch light block's missed txs failed
	ErrFailToFetchLightMissedTxs = errors.New("failed to fetch light block's missed txs")
	// ErrFailToFillLightMissedTxs
	ErrFailToFillLightMissedTxs = errors.New("failed to fill light block's missed txs")

	errInvalidCommittedSeals = errors.New("invalid committed seals")
)

type ByzantineExtra struct {
	Validators    []common.Address
	Seal          []byte         // signature for sealer
	CommittedSeal ByzantineSeals // Pbft signatures, ignore in the Hash calculation
}

type ByzantineSeals = [][]byte

// EncodeRLP serializes ist into the Ethereum RLP format.
func (ist *ByzantineExtra) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{
		ist.Validators,
		ist.Seal,
		ist.CommittedSeal,
	})
}

// DecodeRLP implements rlp.Decoder, and load the istanbul fields from a RLP stream.
func (ist *ByzantineExtra) DecodeRLP(s *rlp.Stream) error {
	var istanbulExtra struct {
		Validators    []common.Address
		Seal          []byte
		CommittedSeal [][]byte
	}
	if err := s.Decode(&istanbulExtra); err != nil {
		return err
	}
	ist.Validators, ist.Seal, ist.CommittedSeal = istanbulExtra.Validators, istanbulExtra.Seal, istanbulExtra.CommittedSeal
	return nil
}

// ExtractByzantineExtra extracts all values of the ByzantineExtra from the header. It returns an
// error if the length of the given extra-data is less than 32 bytes or the extra-data can not
// be decoded.
func ExtractByzantineExtra(h *Header) (*ByzantineExtra, error) {
	if len(h.Extra) < ByzantineExtraVanity {
		return nil, ErrInvalidByzantineHeaderExtra
	}

	var istanbulExtra *ByzantineExtra
	err := rlp.DecodeBytes(h.Extra[ByzantineExtraVanity:], &istanbulExtra)
	if err != nil {
		return nil, err
	}
	return istanbulExtra, nil
}

// writeCommittedSeals writes the extra-data field of a block header with given committed seals.
func WriteCommittedSeals(h *Header, committedSeals ByzantineSeals) error {
	if len(committedSeals) == 0 {
		return errInvalidCommittedSeals
	}

	for _, seal := range committedSeals {
		if len(seal) != ByzantineExtraSeal {
			return errInvalidCommittedSeals
		}
	}

	istanbulExtra, err := ExtractByzantineExtra(h)
	if err != nil {
		return err
	}

	istanbulExtra.CommittedSeal = make([][]byte, len(committedSeals))
	copy(istanbulExtra.CommittedSeal, committedSeals)

	payload, err := rlp.EncodeToBytes(&istanbulExtra)
	if err != nil {
		return err
	}

	h.Extra = append(h.Extra[:ByzantineExtraVanity], payload...)
	return nil
}

// ByzantineFilteredHeader returns a filtered header which some information (like seal, committed seals)
// are clean to fulfill the Istanbul hash rules. It returns nil if the extra-data cannot be
// decoded/encoded by rlp.
func ByzantineFilteredHeader(h *Header, keepSeal bool) *Header {
	newHeader := CopyHeader(h)
	istanbulExtra, err := ExtractByzantineExtra(newHeader)
	if err != nil {
		return nil
	}

	if !keepSeal {
		istanbulExtra.Seal = []byte{}
	}
	istanbulExtra.CommittedSeal = [][]byte{}

	payload, err := rlp.EncodeToBytes(&istanbulExtra)
	if err != nil {
		return nil
	}

	newHeader.Extra = append(newHeader.Extra[:ByzantineExtraVanity], payload...)

	return newHeader
}

func PbftPendingHeader(h *Header, keepSeal bool) *Header {
	newHeader := CopyHeader(h)
	newHeader.Root = common.Hash{}
	newHeader.ReceiptHash = common.Hash{}
	newHeader.Bloom = Bloom{}
	newHeader.GasUsed = 0

	byzantineExtra, err := ExtractByzantineExtra(newHeader)
	if err != nil {
		return nil
	}

	if !keepSeal {
		byzantineExtra.Seal = []byte{}
	}
	byzantineExtra.CommittedSeal = [][]byte{}

	payload, err := rlp.EncodeToBytes(&byzantineExtra)
	if err != nil {
		return nil
	}

	newHeader.Extra = append(newHeader.Extra[:ByzantineExtraVanity], payload...)

	return newHeader
}

func RlpPendingHeaderHash(h *Header) common.Hash {
	return rlpHash([]interface{}{
		h.ParentHash,
		h.UncleHash,
		h.Coinbase,
		h.TxHash,
		h.Difficulty,
		h.Number,
		h.GasLimit,
		h.Time,
		h.Extra,
		h.MixDigest,
		h.Nonce,
	})
}
