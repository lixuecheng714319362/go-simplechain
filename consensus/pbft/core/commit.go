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

	"github.com/simplechain-org/go-simplechain/common"
	"github.com/simplechain-org/go-simplechain/consensus/pbft"
)

func (c *core) sendCommit() {
	sub := c.current.Subject()
	c.broadcastCommit(sub, true)
}

func (c *core) sendCommitForOldBlock(view *pbft.View, pending, digest common.Hash) {
	sub := &pbft.Subject{
		View:    view,
		Pending: pending,
		Digest:  digest,
	}
	c.broadcastCommit(sub, false)
}

func (c *core) broadcastCommit(commit *pbft.Subject, fresh bool) {
	logger := c.logger.New("state", c.state)

	//logger.Error("send commit", "subject", commit, "fresh", fresh)

	encodedSubject, err := Encode(commit)
	if err != nil {
		logger.Error("Failed to encode", "commit", commit)
		return
	}

	commitMsg := &message{
		Code: msgCommit,
		Msg:  encodedSubject,
	}

	//c.broadcast(commitMsg, true)
	c.broadcast(commitMsg, false)

	// if commit is fresh, refresh current state
	if fresh {
		c.acceptCommit(commitMsg)
		c.checkAndCommit(commit)
	}
}

func (c *core) handleCommit(msg *message, src pbft.Validator) error {
	c.commitTimestamp = time.Now()

	// Decode COMMIT message
	var commit *pbft.Subject
	err := msg.Decode(&commit)
	if err != nil {
		return errFailedDecodeCommit
	}

	if err := c.checkMessage(msgCommit, commit.View); err != nil {
		return err
	}

	if err := c.verifyCommit(commit, src); err != nil {
		return err
	}

	c.acceptCommit(msg)
	c.checkAndCommit(commit)
	return nil
}

// verifyCommit verifies if the received COMMIT message is equivalent to our subject
func (c *core) verifyCommit(commit *pbft.Subject, src pbft.Validator) error {
	logger := c.logger.New("from", src, "state", c.state)

	sub := c.current.Subject()
	if !reflect.DeepEqual(commit, sub) {
		logger.Warn("Inconsistent subjects between commit and proposal", "expected", sub, "got", commit)
		return errInconsistentSubject
	}

	return nil
}

func (c *core) acceptCommit(msg *message) error {
	logger := c.logger.New("from", msg.Address, "state", c.state)
	logger.Trace("accept commit msg", "view", c.currentView(), "lockHash", c.current.lockedHash)

	// Add the COMMIT message to current round state
	if err := c.current.Commits.Add(msg); err != nil {
		logger.Error("Failed to record commit message", "msg", msg, "err", err)
		return err
	}

	return nil
}

func (c *core) checkAndCommit(commit *pbft.Subject) {
	// Commit the proposal once we have enough COMMIT messages and we are not in the Committed state.
	//
	// If we already have a proposal, we may have chance to speed up the consensus process
	// by committing the proposal without PREPARE messages.
	if c.current.Commits.Size() >= c.Confirmations() && c.state.Cmp(StateCommitted) < 0 {
		// Still need to call LockHash here since state can skip Prepared state and jump directly to the Committed state.
		c.current.LockHash()
		c.commit()
		return
	}

	// Check and send commit-message, if state is not prepared
	// Sometimes our node received a commit-message from another hashLocked node,
	// the node cannot have enough prepare-messages to upgrade state to prepared or send commit-message
	if c.state.Cmp(StatePrepared) < 0 {
		c.checkAndPrepare(commit)
	}
}
