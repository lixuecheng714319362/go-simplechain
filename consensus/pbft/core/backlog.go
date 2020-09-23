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
	"github.com/simplechain-org/go-simplechain/common/prque"
	"github.com/simplechain-org/go-simplechain/consensus/pbft"
)

var (
	// msgPriority is defined for calculating processing priority to speedup consensus
	// msgPreprepare > msgCommit > msgPrepare
	msgPriority = map[MsgCode]int{
		msgPreprepare: 1,
		msgCommit:     2,
		msgPrepare:    3,
	}
)

// checkMessage checks the message state
// return errInvalidMessage if the message is invalid
// return errFutureMessage if the message view is larger than current view
// return errOldMessage if the message view is smaller than current view
func (c *core) checkMessage(msgCode MsgCode, view *pbft.View) error {
	if view == nil || view.Sequence == nil || view.Round == nil {
		return errInvalidMessage
	}

	if msgCode == msgRoundChange {
		if view.Sequence.Cmp(c.currentView().Sequence) > 0 {
			return errFutureMessage
		} else if view.Cmp(c.currentView()) < 0 {
			return errOldMessage
		}
		return nil
	}

	if view.Cmp(c.currentView()) > 0 {
		return errFutureMessage
	}

	if view.Cmp(c.currentView()) < 0 {
		return errOldMessage
	}

	if c.waitingForRoundChange {
		return errFutureMessage
	}

	if c.state == StateAcceptRequest {
		switch {
		// unsupported light messages node
		case isLightProposalMsg(msgCode) && !c.config.LightMode:
			return errInvalidMessage
		// StateAcceptRequest only accepts msgPreprepare or light proposal messages*
		// other messages are future messages
		case msgCode > msgPreprepare && msgCode <= msgRoundChange:
			return errFutureMessage
		case msgCode == msgGetMissedTxs:
			return errFutureMessage
		}
		return nil
	}

	// For states(StatePreprepared, StatePrepared, StateCommitted),
	// can accept all message types if processing with same view
	return nil
}

func (c *core) storeBacklog(msg *message, src pbft.Validator) {
	logger := c.logger.New("from", src, "state", c.state)

	// dont backlog message from self
	if src.Address() == c.Address() {
		logger.Warn("Backlog from self")
		return
	}

	logger.Trace("Store future message")

	c.backlogsMu.Lock()
	defer c.backlogsMu.Unlock()

	logger.Debug("Retrieving backlog queue", "for", src.Address(), "backlogs_size", len(c.backlogs))
	backlog := c.backlogs[src.Address()]
	if backlog == nil {
		backlog = prque.New(nil)
	}
	switch msg.Code {
	case msgPreprepare:
		var p *pbft.Preprepare
		err := msg.Decode(&p)
		if err == nil {
			backlog.Push(msg, toPriority(msg.Code, p.View))
		}
		// for msgRoundChange, msgPrepare and msgCommit cases
	case msgLightPreprepare:
		var p *pbft.LightPreprepare
		err := msg.Decode(&p)
		if err == nil {
			backlog.Push(msg, toPriority(msg.Code, p.View))
		}
	default:
		var p *pbft.Subject
		err := msg.Decode(&p)
		if err == nil {
			backlog.Push(msg, toPriority(msg.Code, p.View))
		}
	}
	c.backlogs[src.Address()] = backlog
}

func (c *core) processBacklog() {
	c.backlogsMu.Lock()
	defer c.backlogsMu.Unlock()

	for srcAddress, backlog := range c.backlogs {
		if backlog == nil {
			continue
		}
		_, src := c.valSet.GetByAddress(srcAddress)
		if src == nil {
			// validator is not available
			delete(c.backlogs, srcAddress)
			continue
		}
		logger := c.logger.New("from", src, "state", c.state)
		isFuture := false

		// We stop processing if
		//   1. backlog is empty
		//   2. The first message in queue is a future message
		for !(backlog.Empty() || isFuture) {
			m, prio := backlog.Pop()
			msg := m.(*message)
			var view *pbft.View
			switch msg.Code {
			case msgPreprepare:
				var m *pbft.Preprepare
				err := msg.Decode(&m)
				if err == nil {
					view = m.View
				}
			// for msgRoundChange, msgPrepare and msgCommit cases
			case msgLightPreprepare:
				var m *pbft.LightPreprepare
				err := msg.Decode(&m)
				if err == nil {
					view = m.View
				}
			default:
				var sub *pbft.Subject
				err := msg.Decode(&sub)
				if err == nil {
					view = sub.View
				}
			}
			if view == nil {
				logger.Debug("Nil view", "msg", msg)
				continue
			}
			// Push back if it's a future message
			err := c.checkMessage(msg.Code, view)
			if err != nil {
				if err == errFutureMessage {
					logger.Trace("Stop processing backlog", "msg", msg)
					backlog.Push(msg, prio)
					isFuture = true
					break
				}
				logger.Trace("Skip the backlog event", "msg", msg, "err", err)
				continue
			}
			logger.Trace("Post backlog event", "msg", msg)

			go c.sendEvent(backlogEvent{
				src: src,
				msg: msg,
			})
		}
	}
}

func toPriority(msgCode MsgCode, view *pbft.View) int64 {
	if msgCode == msgRoundChange {
		// For msgRoundChange, set the message priority based on its sequence
		return -int64(view.Sequence.Uint64() * 1000)
	}
	// FIXME: round will be reset as 0 while new sequence
	// 10 * Round limits the range of message code is from 0 to 9
	// 1000 * Sequence limits the range of round is from 0 to 99
	return -int64(view.Sequence.Uint64()*1000 + view.Round.Uint64()*10 + uint64(msgPriority[msgCode]))
}
