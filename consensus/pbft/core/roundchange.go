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
	"math/big"

	"github.com/simplechain-org/go-simplechain/common"
	"github.com/simplechain-org/go-simplechain/consensus/pbft"
)

// sendNextRoundChange sends the ROUND CHANGE message with current round + 1
func (c *core) sendNextRoundChange() {
	cv := c.currentView()
	c.sendRoundChange(new(big.Int).Add(cv.Round, common.Big1))
}

// sendRoundChange sends the ROUND CHANGE message with the given round
func (c *core) sendRoundChange(round *big.Int) {
	logger := c.logger.New("state", c.state)

	cv := c.currentView()
	if cv.Round.Cmp(round) >= 0 {
		logger.Error("Cannot send out the round change", "current round", cv.Round, "target round", round)
		return
	}

	c.catchUpRound(&pbft.View{
		// The round number we'd like to transfer to.
		Round:    new(big.Int).Set(round),
		Sequence: new(big.Int).Set(cv.Sequence),
	})

	// Now we have the new round number and sequence number
	cv = c.currentView()
	rc := &pbft.Subject{
		View: cv,
	}

	payload, err := Encode(rc)
	if err != nil {
		logger.Error("Failed to encode ROUND CHANGE", "rc", rc, "err", err)
		return
	}

	rcMsg := &message{
		Code: msgRoundChange,
		Msg:  payload,
	}

	//c.broadcast(rcMsg, true)
	c.broadcast(rcMsg, false)
	c.acceptAndCheckRoundChange(rcMsg, rc)
}

func (c *core) handleRoundChange(msg *message, src pbft.Validator) error {
	logger := c.logger.New("state", c.state, "from", src.Address().Hex())

	// Decode ROUND CHANGE message
	var rc *pbft.Subject
	if err := msg.Decode(&rc); err != nil {
		logger.Error("Failed to decode ROUND CHANGE", "err", err)
		return errInvalidMessage
	}

	if err := c.checkMessage(msgRoundChange, rc.View); err != nil {
		return err
	}

	return c.acceptAndCheckRoundChange(msg, rc)
}

func (c *core) acceptAndCheckRoundChange(msg *message, rc *pbft.Subject) error {
	logger := c.logger.New("state", c.state, "from", msg.Address)
	//logger.Trace("accept round change", "view", c.currentView())

	cv := c.currentView()
	roundView := rc.View

	// Add the ROUND CHANGE message to its message set and return how many
	// messages we've got with the same round number and sequence number.
	num, err := c.roundChangeSet.Add(roundView.Round, msg)

	if err != nil {
		logger.Warn("Failed to add round change message", "msg", msg, "err", err)
		return err
	}

	// Once we received f+1 ROUND CHANGE messages, those messages form a weak certificate.
	// If our round number is smaller than the certificate's round number, we would
	// try to catch up the round number.
	if c.waitingForRoundChange && num == c.valSet.F()+1 {
		if cv.Round.Cmp(roundView.Round) < 0 {
			c.sendRoundChange(roundView.Round)
		}
		return nil
	} else if num == c.Confirmations() && (c.waitingForRoundChange || cv.Round.Cmp(roundView.Round) < 0) {
		// We've received 2f+1/Ceil(2N/3) ROUND CHANGE messages, start a new round immediately.
		c.startNewRound(roundView.Round)
		return nil
	} else if cv.Round.Cmp(roundView.Round) < 0 {
		// Only gossip the message with current round to other validators.
		return errIgnored
	}

	return nil
}
