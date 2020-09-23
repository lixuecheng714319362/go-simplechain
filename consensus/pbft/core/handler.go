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
	"github.com/simplechain-org/go-simplechain/common"
	"github.com/simplechain-org/go-simplechain/consensus/pbft"
	"github.com/simplechain-org/go-simplechain/log"
)

// Start implements core.Engine.Start
func (c *core) Start() error {
	// Start a new round from last sequence + 1
	c.startNewRound(common.Big0)

	// Tests will handle events itself, so we have to make subscribeEvents()
	// be able to call in test.
	c.subscribeEvents()
	go c.handleEvents()

	return nil
}

// Stop implements core.Engine.Stop
func (c *core) Stop() error {
	c.stopTimer()
	c.unsubscribeEvents()

	// Make sure the handler goroutine exits
	c.handlerWg.Wait()
	return nil
}

// ----------------------------------------------------------------------------

// Subscribe both internal and external events
func (c *core) subscribeEvents() {
	c.events = c.backend.EventMux().Subscribe(
		// external events
		pbft.RequestEvent{},
		pbft.MessageEvent{},
		// internal events
		backlogEvent{},
	)
	c.timeoutSub = c.backend.EventMux().Subscribe(
		timeoutEvent{},
	)
	c.finalCommittedSub = c.backend.EventMux().Subscribe(
		pbft.FinalCommittedEvent{},
	)
}

// Unsubscribe all events
func (c *core) unsubscribeEvents() {
	c.events.Unsubscribe()
	c.timeoutSub.Unsubscribe()
	c.finalCommittedSub.Unsubscribe()
}

func (c *core) handleEvents() {
	// Clear state
	defer func() {
		c.current = nil
		c.handlerWg.Done()
	}()

	c.handlerWg.Add(1)

	for {
		select {
		case event, ok := <-c.events.Chan():
			if !ok {
				return
			}
			// A real event arrived, process interesting content
			switch ev := event.Data.(type) {
			case pbft.RequestEvent:
				r := &pbft.Request{
					Proposal: ev.Proposal,
				}
				err := c.handleRequest(r)
				if err == errFutureMessage {
					c.storeRequestMsg(r)
				}
			case pbft.MessageEvent:
				msg, src, forward, err := c.handleMsg(ev.Payload)
				if forward && err == nil {
					c.forward(msg, src)
				}
			case backlogEvent:
				// No need to check signature for internal messages
				if forward, err := c.handleCheckedMsg(ev.msg, ev.src); forward && err == nil {
					p, err := ev.msg.Payload()
					if err != nil {
						c.logger.Warn("Get message payload failed", "err", err)
						continue
					}
					//c.backend.Gossip(c.valSet, p)
					c.backend.Guidance(c.valSet, ev.msg.Address, p)
				}
			}
		case _, ok := <-c.timeoutSub.Chan():
			if !ok {
				return
			}
			c.handleTimeoutMsg()

		case event, ok := <-c.finalCommittedSub.Chan():
			if !ok {
				return
			}
			switch ev := event.Data.(type) {
			case pbft.FinalCommittedEvent:
				c.handleFinalCommitted(ev.Committed)
			}
		}
	}
}

// sendEvent sends events to mux
func (c *core) sendEvent(ev interface{}) {
	c.backend.EventMux().Post(ev)
}

func (c *core) handleMsg(payload []byte) (*message, pbft.Validator, bool, error) {
	logger := c.logger.New()

	// Decode message and check its signature
	msg := new(message)
	if err := msg.FromPayload(payload, c.validateFn); err != nil {
		logger.Error("Failed to decode message from payload", "err", err)
		return nil, nil, false, err
	}

	// Only accept message if the address is valid
	_, src := c.valSet.GetByAddress(msg.Address)
	if src == nil {
		logger.Error("Invalid address in message", "msg", msg)
		return msg, src, false, pbft.ErrUnauthorizedAddress
	}

	forward, err := c.handleCheckedMsg(msg, src)

	return msg, src, forward, err
}

func (c *core) handleCheckedMsg(msg *message, src pbft.Validator) (bool, error) {
	logger := c.logger.New("address", c.address, "from", src)

	// Store the message if it's a future message
	testBacklog := func(err error) error {
		if err == errFutureMessage {
			c.storeBacklog(msg, src)
		}

		return err
	}

	switch msg.Code {
	case msgPreprepare:
		// wouldn't forward preprepare message, if node is in light mode
		return !c.config.LightMode, testBacklog(c.handlePreprepare(msg, src))

	case msgPrepare:
		return true, testBacklog(c.handlePrepare(msg, src))

	case msgCommit:
		return true, testBacklog(c.handleCommit(msg, src))

	case msgRoundChange:
		return true, testBacklog(c.handleRoundChange(msg, src))

	case msgLightPreprepare:
		log.Report("> handleCheckedMsg msgLightPreprepare")
		if c.config.LightMode {
			return true, testBacklog(c.handleLightPrepare(msg, src))
		}
	case msgGetMissedTxs:
		if c.config.LightMode {
			// wouldn't forward request message
			return false, testBacklog(c.handleGetMissedTxs(msg, src))
		}
	case msgMissedTxs:
		if c.config.LightMode {
			// wouldn't forward response message
			return false, testBacklog(c.handleMissedTxs(msg, src))
		}

	default:
		logger.Error("Invalid message", "msg", msg)
	}

	return false, errInvalidMessage
}

func (c *core) handleTimeoutMsg() {
	// adjust block sealing txs on timeout
	c.backend.OnTimeout()
	// If we're not waiting for round change yet, we can try to catch up
	// the max round with F+1 round change message. We only need to catch up
	// if the max round is larger than current round.
	if !c.waitingForRoundChange {
		maxRound := c.roundChangeSet.MaxRound(c.valSet.F() + 1)
		if maxRound != nil && maxRound.Cmp(c.current.Round()) > 0 {
			c.sendRoundChange(maxRound)
			return
		}
	}

	lastProposal, _, _ := c.backend.LastProposal()
	if lastProposal != nil && lastProposal.Number().Cmp(c.current.Sequence()) >= 0 {
		c.logger.Trace("round change timeout, catch up latest sequence", "number", lastProposal.Number().Uint64())
		c.startNewRound(common.Big0)
	} else {
		c.sendNextRoundChange()
	}
}
