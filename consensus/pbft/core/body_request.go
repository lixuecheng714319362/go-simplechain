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
	"github.com/simplechain-org/go-simplechain/consensus/pbft"
	"github.com/simplechain-org/go-simplechain/core/types"
)

func (c *core) requestMissedTxs(missedTxs []types.MissedTx, val pbft.Validator) {
	logger := c.logger.New("state", c.state, "to", val)

	missedReq := &pbft.MissedReq{
		View:      c.currentView(),
		MissedTxs: missedTxs,
	}

	encMissedReq, err := Encode(missedReq)
	if err != nil {
		logger.Error("Failed to encode", "missedReq", missedReq, "err", err)
		return
	}

	//logger.Trace("[report] requestMissedTxs", "view", missedReq.View, "missed", len(missedTxs))

	c.send(&message{
		Code: msgGetMissedTxs,
		Msg:  encMissedReq,
	}, pbft.Validators{val})
}

func (c *core) handleGetMissedTxs(msg *message, src pbft.Validator) error {
	logger := c.logger.New("from", src, "state", c.state)

	var missed *pbft.MissedReq
	err := msg.Decode(&missed)
	if err != nil {
		logger.Error("Failed to decode", "err", err)
		return errFailedDecodePrepare
	}

	//logger.Trace("[report] handleGetMissedTxs", "view", missed.View, "missed", len(missed.MissedTxs))

	if err := c.checkMessage(msgGetMissedTxs, missed.View); err != nil {
		logFn := logger.Warn
		switch err {
		case errOldMessage: //TODO
			logFn = logger.Trace
		case errFutureMessage: //TODO
			logFn = logger.Trace
		}
		logFn("GetMissedTxs checkMessage failed", "view", missed.View, "missed", len(missed.MissedTxs), "err", err)
		return err
	}

	// proposer must have a filled proposal, return if the proposal is not exist
	proposal := c.current.Proposal()
	if proposal == nil {
		logger.Warn("nonexistent completed proposal")
		return errNonexistentProposal
	}

	txs, err := proposal.FetchMissedTxs(missed.MissedTxs)
	if err != nil {
		return err
	}

	c.responseMissedTxs(txs, src)

	return nil
}

func (c *core) responseMissedTxs(txs types.Transactions, val pbft.Validator) {
	logger := c.logger.New("state", c.state, "to", val)

	missedResp := &pbft.MissedResp{
		View:   c.currentView(),
		ReqTxs: txs,
	}

	//logger.Trace("[report] responseMissedTxs", "view", missedResp.View, "missed", len(txs))

	//encMissedResp, err := Encode(missedResp)
	encMissedResp, err := missedResp.EncodeOffset()
	if err != nil {
		logger.Error("Failed to encode", "missedResp", missedResp, "err", err)
		return
	}

	// Mark txs known by val, and do not sync them again
	c.backend.MarkTransactionKnownBy(val, txs)

	c.send(&message{
		Code: msgMissedTxs,
		Msg:  encMissedResp,
	}, pbft.Validators{val})
}

func (c *core) handleMissedTxs(msg *message, src pbft.Validator) error {
	logger := c.logger.New("from", src, "state", c.state)

	var missed pbft.MissedResp
	//err := msg.Decode(&missed)
	err := missed.DecodeOffset(msg.Msg)
	if err != nil {
		return errFailedDecodePrepare
	}

	//logger.Trace("[report] handleMissedTxs", "view", missed.View)

	if err := c.checkMessage(msgMissedTxs, missed.View); err != nil {
		logFn := logger.Warn
		switch err {
		case errOldMessage: //TODO
			logFn = logger.Trace
		case errFutureMessage: //TODO
			logFn = logger.Trace
		}
		logFn("MissedTxs checkMessage failed", "view", missed.View, "missed", len(missed.ReqTxs), "err", err)
		return err
	}

	lp := c.current.LightProposal()
	if lp == nil {
		logger.Warn("local light proposal was lost", "view", missed.View, "Preprepare", c.current.Preprepare)
		return nil
	}

	// do not accept completed proposal repeatedly
	if lp.Completed() {
		logger.Warn("local light was already completed", "view", missed.View)
		return nil
	}

	if err := lp.FillMissedTxs(missed.ReqTxs); err != nil {
		return err
	}

	return c.handleLightPrepare2(c.current.LightPrepare.FullPreprepare(), src)
}
