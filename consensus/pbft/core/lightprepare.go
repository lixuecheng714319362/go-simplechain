package core

import (
	"time"

	"github.com/simplechain-org/go-simplechain/consensus"
	"github.com/simplechain-org/go-simplechain/consensus/pbft"
	"github.com/simplechain-org/go-simplechain/log"
)

func (c *core) sendLightPrepare(request *pbft.Request, curView *pbft.View) {
	logger := c.logger.New("state", c.state)

	// encode light proposal
	lightMsg, err := Encode(&pbft.Preprepare{
		View:     curView,
		Proposal: pbft.Proposal2Light(request.Proposal, true),
	})
	if err != nil {
		logger.Error("Failed to encode", "view", curView)
		return
	}

	// send light pre-prepare msg to others
	c.broadcast(&message{
		Code: msgLightPreprepare,
		Msg:  lightMsg,
	}, false)

	// handle full proposal by itself
	c.handlePrepare2(&pbft.Preprepare{
		View:     curView,
		Proposal: request.Proposal,
	}, nil, nil)
}

// The first stage handle light Pre-prepare.
// Check message and verify block header, and try fill proposal with sealer.
// Request missed txs from proposer or enter the second stage for filled proposal.
func (c *core) handleLightPrepare(msg *message, src pbft.Validator) error {
	logger := c.logger.New("from", src, "state", c.state)
	c.prepareTimestamp = time.Now()

	var preprepare *pbft.LightPreprepare
	err := msg.Decode(&preprepare)
	if err != nil {
		logger.Warn("Failed to decode light preprepare", "err", err)
		return errFailedDecodePreprepare
	}

	err = c.checkPreprepareMsg(msg, src, preprepare.View, preprepare.Proposal)
	if err != nil {
		return err
	}

	// Verify the proposal we received, dont check body if we are light
	if duration, err := c.backend.Verify(preprepare.Proposal, true, false); err != nil {
		// if it's a future block, we will handle it again after the duration
		if err == consensus.ErrFutureBlock {
			if duration > time.Second*time.Duration(c.config.BlockPeriod) {
				logger.Warn("Proposed block will be committed in the future", "err", err, "duration", duration)
				// wait until block timestamp at commit stage
				c.stopFuturePreprepareTimer()
				c.futurePreprepareTimer = time.AfterFunc(duration, func() {
					c.sendEvent(backlogEvent{
						src: src,
						msg: msg,
					})
				})
			}
		} else {
			logger.Warn("Failed to verify light proposal header", "err", err, "duration", duration)
			c.sendNextRoundChange()
			return err
		}
	}

	if c.state != StateAcceptRequest {
		return nil
	}

	lightProposal, ok := preprepare.Proposal.(pbft.LightProposal)
	if !ok {
		logger.Warn("Failed resolve proposal as a light proposal", "view", preprepare.View)
		return errInvalidLightProposal
	}

	// empty block, handle immediately
	if len(lightProposal.TxDigests()) == 0 {
		return c.handleLightPrepare2(preprepare.FullPreprepare(), src)
	}

	// fill light proposal by txpool, return missed transactions
	filled, missedTxs, err := c.backend.FillLightProposal(lightProposal)
	if err != nil {
		logger.Warn("Failed to fill light proposal", "error", err)
		c.sendNextRoundChange()
		return err
	}

	// cal percent of txs existed in local txpool
	percent := 100.00 - 100.00*float64(len(missedTxs))/float64(len(lightProposal.TxDigests()))
	logger.Trace("light block transaction covered", "percent", percent)

	// 1.block filled, handle immediately
	// 2.block missed some txs, request them
	if filled {
		// entire the second stage
		return c.handleLightPrepare2(preprepare.FullPreprepare(), src)

	} else {
		// accept light preprepare
		c.current.SetLightPrepare(preprepare)
		// request missedTxs from proposer
		c.requestMissedTxs(missedTxs, src)
	}

	return nil
}

// The second stage handle light Pre-prepare.
func (c *core) handleLightPrepare2(preprepare *pbft.Preprepare, src pbft.Validator) error {
	logger := c.logger.New("from", src, "state", c.state)
	log.Report("> handleLightPrepare2")
	// light proposal was be filled, check body
	if _, err := c.backend.Verify(preprepare.Proposal, false, true); err != nil {
		logger.Warn("Failed to verify light proposal body", "err", err)
		c.sendNextRoundChange()
		return err //TODO
	}
	return c.checkAndAcceptPreprepare(preprepare)
}
