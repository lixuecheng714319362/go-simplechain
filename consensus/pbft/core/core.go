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
	"bytes"
	"math"
	"math/big"
	"sync"
	"time"

	"github.com/simplechain-org/go-simplechain/common"
	cmath "github.com/simplechain-org/go-simplechain/common/math"
	"github.com/simplechain-org/go-simplechain/common/prque"
	"github.com/simplechain-org/go-simplechain/consensus/pbft"
	"github.com/simplechain-org/go-simplechain/core/types"
	"github.com/simplechain-org/go-simplechain/event"
	"github.com/simplechain-org/go-simplechain/log"
	"github.com/simplechain-org/go-simplechain/metrics"
)

// New creates an Istanbul consensus core
func New(backend pbft.Backend, config *pbft.Config) Engine {
	//r := metrics.NewRegistry()
	c := &core{
		config:            config,
		address:           backend.Address(),
		state:             StateAcceptRequest,
		handlerWg:         new(sync.WaitGroup),
		logger:            log.New("address", backend.Address()),
		backend:           backend,
		backlogs:          make(map[common.Address]*prque.Prque),
		backlogsMu:        new(sync.Mutex),
		pendingRequests:   prque.New(nil),
		pendingRequestsMu: new(sync.Mutex),
	}

	c.roundMeter = metrics.NewRegisteredMeter("consensus/pbft/core/round", nil)
	c.sequenceMeter = metrics.NewRegisteredMeter("consensus/pbft/core/sequence", nil)
	c.consensusTimer = metrics.NewRegisteredTimer("consensus/pbft/core/consensus", nil)

	//r.Register("consensus/pbft/core/round", c.roundMeter)
	//r.Register("consensus/pbft/core/sequence", c.sequenceMeter)
	//r.Register("consensus/pbft/core/consensus", c.consensusTimer)

	c.validateFn = c.checkValidatorSignature
	return c
}

// ----------------------------------------------------------------------------

const (
	timeoutRate     = 1.3
	maxRoundTimeout = 10
)

type core struct {
	config  *pbft.Config
	address common.Address
	state   State
	logger  log.Logger

	backend               pbft.Backend
	events                *event.TypeMuxSubscription
	finalCommittedSub     *event.TypeMuxSubscription
	timeoutSub            *event.TypeMuxSubscription
	futurePreprepareTimer *time.Timer

	valSet                pbft.ValidatorSet
	waitingForRoundChange bool
	validateFn            func([]byte, []byte) (common.Address, error)

	backlogs   map[common.Address]*prque.Prque
	backlogsMu *sync.Mutex

	current   *roundState
	handlerWg *sync.WaitGroup

	roundChangeSet   *roundChangeSet
	roundChangeTimer *time.Timer

	pendingRequests   *prque.Prque
	pendingRequestsMu *sync.Mutex

	// metrics

	// the meter to record the round change rate
	roundMeter metrics.Meter
	// the meter to record the sequence update rate
	sequenceMeter metrics.Meter
	// the timer to record consensus duration (from accepting a preprepare to final committed stage)
	consensusTimer     metrics.Timer
	consensusTimestamp time.Time

	preprepareTimer     metrics.Timer
	preprepareTimestamp time.Time

	executeTimer     metrics.Timer
	executeTimestamp time.Time

	prepareTimer     metrics.Timer
	prepareTimestamp time.Time

	commitTimer     metrics.Timer
	commitTimestamp time.Time
}

func (c *core) finalizeMessage(msg *message) ([]byte, error) {
	var err error
	// Add sender address
	msg.Address = c.Address()

	// Add proof of consensus
	msg.CommittedSeal = []byte{}
	// Assign the CommittedSeal if it's a COMMIT message and proposal is not nil
	if msg.Code == msgCommit && c.current.Conclusion() != nil {
		seal := PrepareCommittedSeal(c.current.Conclusion().Hash())
		//seal := PrepareCommittedSeal(c.current.Proposal().PendingHash()) //TODO: 签pendingHash(proposal)还是Conclusion ？
		msg.CommittedSeal, err = c.backend.Sign(seal)
		if err != nil {
			return nil, err
		}
	}

	// Sign message
	data, err := msg.PayloadNoSig()
	if err != nil {
		return nil, err
	}
	msg.Signature, err = c.backend.Sign(data)
	if err != nil {
		return nil, err
	}

	// Convert to payload
	payload, err := msg.Payload()
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func (c *core) _broadcast(msg *message, self bool) {
	logger := c.logger.New("state", c.state)

	payload, err := c.finalizeMessage(msg)
	if err != nil {
		logger.Error("Failed to finalize message", "msg", msg, "err", err)
		return
	}

	// Broadcast payload
	if err = c.backend.Broadcast(c.valSet, msg.Address, payload); err != nil {
		logger.Error("Failed to broadcast message", "msg", msg, "err", err)
	}
	// Post payload
	if self {
		c.backend.Post(payload)
	}
}

func (c *core) broadcast(msg *message, self bool) {
	logger := c.logger.New("state", c.state)

	ps, nodes := c.backend.GetForwardNodes(c.valSet.List())
	// finalize forward nodes too
	msg.ForwardNodes = nodes
	payload, err := c.finalizeMessage(msg)
	if err != nil {
		logger.Error("Failed to finalize message", "msg", msg, "err", err)
		return
	}

	// Broadcast payload
	if err = c.backend.BroadcastMsg(ps, msg.Hash(), payload); err != nil {
		//if err = c.backend.Broadcast(c.valSet, msg.Address, payload); err != nil {
		logger.Error("Failed to broadcast message", "msg", msg, "err", err)
	}
	// Post payload
	if self {
		c.backend.Post(payload)
	}
}

func (c *core) send(msg *message, val pbft.Validators) {
	logger := c.logger.New("state", c.state)

	payload, err := c.finalizeMessage(msg)
	if err != nil {
		logger.Error("Failed to finalize message", "msg", msg, "err", err)
		return
	}

	if err = c.backend.SendMsg(val, payload); err != nil {
		logger.Error("Failed to send message", "msg", msg, "err", err)
		return
	}
}

func (c *core) forward(msg *message, src pbft.Validator) bool {
	logger := c.logger.New("state", c.state, "from", src)

	forwardNodes := msg.ForwardNodes
	// no nodes need to forward, exit
	if forwardNodes == nil {
		return false
	}
	var forwardValidator pbft.Validators
	for _, forward := range forwardNodes {
		if i, val := c.valSet.GetByAddress(forward); i >= 0 {
			forwardValidator = append(forwardValidator, val)
		} else {
			logger.Warn("invalid forward node", "address", forward)
		}
	}

	ps, remainNodes := c.backend.GetForwardNodes(forwardValidator)
	// no forward peers existed in protocol, exit
	if ps == nil {
		return false
	}

	// create message with new forwardNodes
	msg.ForwardNodes = remainNodes
	payload, err := msg.Payload()
	if err != nil {
		logger.Error("Failed to forward message", "msg", msg, "err", err)
		return false
	}

	if err = c.backend.BroadcastMsg(ps, msg.Hash(), payload); err != nil {
		logger.Error("Failed to forward message", "msg", msg, "err", err)
		return false
	}

	return true
}

func (c *core) currentView() *pbft.View {
	return &pbft.View{
		Sequence: new(big.Int).Set(c.current.Sequence()),
		Round:    new(big.Int).Set(c.current.Round()),
	}
}

func (c *core) IsProposer() bool {
	v := c.valSet
	if v == nil {
		return false
	}
	return v.IsProposer(c.backend.Address())
}

func (c *core) IsCurrentProposal(proposalHash common.Hash) bool {
	//return c.current != nil && c.current.pendingRequest != nil && c.current.pendingRequest.Proposal.Hash() == blockHash
	return c.current != nil && c.current.pendingRequest != nil && c.current.pendingRequest.Proposal.PendingHash() == proposalHash
}

func (c *core) commit() {
	c.setState(StateCommitted)

	conclusion := c.current.Conclusion()
	if conclusion != nil {
		committedSeals := make([][]byte, c.current.Commits.Size())
		for i, v := range c.current.Commits.Values() {
			committedSeals[i] = make([]byte, types.IstanbulExtraSeal)
			copy(committedSeals[i][:], v.CommittedSeal[:])
		}

		if err := c.backend.Commit(conclusion, committedSeals); err != nil {
			c.logger.Error("Commit Failed", "conclusion", conclusion.Hash(), "num", conclusion.Number(), "err", err)
			c.current.UnlockHash() //Unlock block when insertion fails
			c.sendNextRoundChange()
		}
	}

}

// startNewRound starts a new round. if round equals to 0, it means to starts a new sequence
func (c *core) startNewRound(round *big.Int) {
	var logger log.Logger
	if c.current == nil {
		logger = c.logger.New("old_round", -1, "old_seq", 0)
	} else {
		logger = c.logger.New("old_round", c.current.Round(), "old_seq", c.current.Sequence())
	}

	roundChange := false
	// Try to get last proposal(conclusion)
	_, lastProposal, lastProposer := c.backend.LastProposal()
	if c.current == nil {
		logger.Trace("Start to the initial round")

	} else if lastProposal.Number().Cmp(c.current.Sequence()) >= 0 {
		diff := new(big.Int).Sub(lastProposal.Number(), c.current.Sequence())
		c.sequenceMeter.Mark(new(big.Int).Add(diff, common.Big1).Int64())

		if !c.consensusTimestamp.IsZero() {
			c.consensusTimer.UpdateSince(c.consensusTimestamp)
			c.consensusTimestamp = time.Time{}
		}
		//logger.Trace("Catch up latest proposal", "number", lastProposal.Number().Uint64(), "hash", lastProposal.Hash())
		logger.Trace("Catch up latest proposal", "number", lastProposal.Number().Uint64(), "hash", lastProposal.PendingHash())

	} else if lastProposal.Number().Cmp(big.NewInt(c.current.Sequence().Int64()-1)) == 0 {
		if round.Cmp(common.Big0) == 0 {
			// same seq and round, don't need to start new round
			return
		} else if round.Cmp(c.current.Round()) < 0 {
			logger.Warn("New round should not be smaller than current round", "seq", lastProposal.Number().Int64(), "new_round", round, "old_round", c.current.Round())
			return
		}
		roundChange = true

	} else {
		logger.Warn("New sequence should be larger than current sequence", "new_seq", lastProposal.Number().Int64())
		return
	}

	var newView *pbft.View
	if roundChange {
		newView = &pbft.View{
			Sequence: new(big.Int).Set(c.current.Sequence()),
			Round:    new(big.Int).Set(round),
		}
	} else {
		newView = &pbft.View{
			Sequence: new(big.Int).Add(lastProposal.Number(), common.Big1),
			Round:    new(big.Int),
		}
		c.valSet = c.backend.Validators(lastProposal)
	}

	// Update logger
	logger = logger.New("old_proposer", c.valSet.GetProposer())
	// Clear invalid ROUND CHANGE messages
	c.roundChangeSet = newRoundChangeSet(c.valSet)
	// New snapshot for new round
	c.updateRoundState(newView, c.valSet, roundChange)
	// Calculate new proposer
	c.valSet.CalcProposer(lastProposer, newView.Round.Uint64())
	c.waitingForRoundChange = false
	c.setState(StateAcceptRequest)
	if roundChange && c.IsProposer() && c.current != nil {
		// If it is locked, propose the old proposal
		// If we have pending request, propose pending request
		if c.current.IsHashLocked() {
			r := &pbft.Request{
				Proposal: c.current.Proposal(), //c.current.Proposal would be the locked proposal by previous proposer, see updateRoundState
			}
			c.sendPreprepare(r)
		} else if c.current.pendingRequest != nil {
			c.sendPreprepare(c.current.pendingRequest)
		}
	}
	c.newRoundChangeTimer()

	logger.Debug("New round", "new_round", newView.Round, "new_seq", newView.Sequence, "new_proposer", c.valSet.GetProposer(), "valSet", c.valSet.List(), "size", c.valSet.Size(), "IsProposer", c.IsProposer())
}

func (c *core) catchUpRound(view *pbft.View) {
	logger := c.logger.New("old_round", c.current.Round(), "old_seq", c.current.Sequence(), "old_proposer", c.valSet.GetProposer())

	if view.Round.Cmp(c.current.Round()) > 0 {
		c.roundMeter.Mark(new(big.Int).Sub(view.Round, c.current.Round()).Int64())
	}
	c.waitingForRoundChange = true

	// Need to keep block locked for round catching up
	c.updateRoundState(view, c.valSet, true)
	c.roundChangeSet.Clear(view.Round)
	c.newRoundChangeTimer()

	logger.Trace("Catch up round", "new_round", view.Round, "new_seq", view.Sequence, "new_proposer", c.valSet)
}

// updateRoundState updates round state by checking if locking block is necessary
func (c *core) updateRoundState(view *pbft.View, validatorSet pbft.ValidatorSet, roundChange bool) {
	// Lock only if both roundChange is true and it is locked
	if roundChange && c.current != nil {
		if c.current.IsHashLocked() {
			c.current = newRoundState(view, validatorSet, c.current.GetLockedHash(), c.current.Preprepare, c.current.Prepare, c.current.pendingRequest, c.backend.HasBadProposal)
		} else {
			c.current = newRoundState(view, validatorSet, common.Hash{}, nil, nil, c.current.pendingRequest, c.backend.HasBadProposal)
		}
	} else {
		c.current = newRoundState(view, validatorSet, common.Hash{}, nil, nil, nil, c.backend.HasBadProposal)
	}
}

func (c *core) setState(state State) {
	if c.state != state {
		c.state = state
	}
	if state == StateAcceptRequest {
		c.processPendingRequests()
	}
	c.processBacklog()
}

func (c *core) Address() common.Address {
	return c.address
}

func (c *core) stopFuturePreprepareTimer() {
	if c.futurePreprepareTimer != nil {
		c.futurePreprepareTimer.Stop()
	}
}

func (c *core) stopTimer() {
	c.stopFuturePreprepareTimer()
	if c.roundChangeTimer != nil {
		c.roundChangeTimer.Stop()
	}
}

func (c *core) newRoundChangeTimer() {
	c.stopTimer()

	// set timeout based on the round number
	round := cmath.Uint64Min(c.current.Round().Uint64(), maxRoundTimeout)
	timeout := time.Duration(c.config.RequestTimeout) * time.Millisecond * time.Duration(math.Pow(timeoutRate, float64(round)))

	c.roundChangeTimer = time.AfterFunc(timeout, func() {
		log.Warn("timeout, send view change", "timeout", timeout, "round", round)
		c.sendEvent(timeoutEvent{}) //FIXME: send timeoutEvent
	})
}

func (c *core) checkValidatorSignature(data []byte, sig []byte) (common.Address, error) {
	return pbft.CheckValidatorSignature(c.valSet, data, sig)
}

func (c *core) Confirmations() int {
	// Confirmation Formula used ceil(2N/3)
	return int(math.Ceil(float64(2*c.valSet.Size()) / 3))
}

// PrepareCommittedSeal returns a committed seal for the given hash
func PrepareCommittedSeal(hash common.Hash) []byte {
	var buf bytes.Buffer
	buf.Write(hash.Bytes())
	buf.Write([]byte{byte(msgCommit)})
	return buf.Bytes()
}
