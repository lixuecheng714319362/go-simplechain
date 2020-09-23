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
	"reflect"
	"time"

	"github.com/simplechain-org/go-simplechain/consensus/pbft"
)

func (c *core) sendPrepare() {

	logger := c.logger.New("state", c.state)

	sub := c.current.Subject()

	encodedSubject, err := Encode(sub)
	if err != nil {
		logger.Error("Failed to encode", "subject", sub)
		return
	}

	prepareMsg := &message{
		Code: msgPrepare,
		Msg:  encodedSubject,
	}

	c.broadcast(prepareMsg, false)

	//_, src := c.valSet.GetByAddress(c.address)
	//c.handlePrepare(prepareMsg, src)
	c.acceptPrepare(prepareMsg, sub.View)
	c.checkAndCommitPrepare(sub)
}

func (c *core) handlePrepare(msg *message, src pbft.Validator) error {
	c.prepareTimestamp = time.Now()

	// Decode PsREPARE message
	var prepare *pbft.Subject
	err := msg.Decode(&prepare)
	if err != nil {
		return errFailedDecodePrepare
	}

	if err := c.checkMessage(msgPrepare, prepare.View); err != nil {
		return err
	}
	// If it is locked, it can only process on the locked block.
	// Passing verifyPrepare and checkMessage implies it is processing on the locked block since it was verified in the Preprepared state.
	if err := c.verifyPrepare(prepare, src); err != nil {
		return err
	}
	c.acceptPrepare(msg, prepare.View)
	c.checkAndCommitPrepare(prepare)

	return nil
}

// verifyPrepare verifies if the received PREPARE message is equivalent to our subject
func (c *core) verifyPrepare(prepare *pbft.Subject, src pbft.Validator) error {
	logger := c.logger.New("from", src, "state", c.state)

	sub := c.current.Subject()
	if !reflect.DeepEqual(prepare, sub) {
		logger.Warn("Inconsistent subjects between PREPARE and proposal", "expected", sub, "got", prepare)
		return errInconsistentSubject
	}

	return nil
}

func (c *core) acceptPrepare(msg *message, view *pbft.View) error {
	logger := c.logger.New("from", msg.Address, "state", c.state)
	//logger.Trace("accept prepare msg", "view", view, "lockHash", c.current.lockedHash)

	// Add the PREPARE message to current round state
	if err := c.current.Prepares.Add(msg); err != nil {
		logger.Error("Failed to add PREPARE message to round state", "msg", msg, "err", err)
		return err
	}

	return nil
}

func (c *core) checkAndCommitPrepare(prepare *pbft.Subject) {
	// Change to Prepared state if we've received enough PREPARE messages or it is locked
	// and we are in earlier state before Prepared state.
	if (
		(c.current.IsHashLocked() && prepare.Digest == c.current.GetLockedHash()) ||
			c.current.GetPrepareOrCommitSize() >= c.Confirmations()) && c.state.Cmp(StatePrepared) < 0 {
		c.current.LockHash()
		c.setState(StatePrepared)
		c.sendCommit()
	}
}
