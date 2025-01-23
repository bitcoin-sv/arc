package metamorph

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/libsv/go-p2p/chaincfg/chainhash"

	"github.com/bitcoin-sv/arc/internal/logger"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
)

var allowedTopics = []string{
	"hashblock",
	"hashblock2",
	"hashtx",
	"hashtx2",
	"rawblock",
	"rawblock2",
	"rawtx",
	"rawtx2",
	"discardedfrommempool",
	"removedfrommempoolblock",
	"invalidtx",
}

var ErrNilZMQHandler = errors.New("zmq handler is nil")

type subscriptionRequest struct {
	topic string
	ch    chan []string
}

type ZMQ struct {
	url             *url.URL
	statusMessageCh chan<- *TxStatusMessage
	handler         ZMQI
	logger          *slog.Logger
}

type ZMQTxInfo struct {
	TxID                        string        `json:"txid"`
	FromBlock                   bool          `json:"fromBlock"`
	Source                      string        `json:"source"`
	Address                     string        `json:"address"`
	NodeID                      int           `json:"nodeId"`
	Size                        int           `json:"size"`
	Hex                         string        `json:"hex"`
	IsInvalid                   bool          `json:"isInvalid"`
	IsValidationError           bool          `json:"isValidationError"`
	IsMissingInputs             bool          `json:"isMissingInputs"`
	IsDoubleSpendDetected       bool          `json:"isDoubleSpendDetected"`
	IsMempoolConflictDetected   bool          `json:"isMempoolConflictDetected"`
	IsNonFinal                  bool          `json:"isNonFinal"`
	IsValidationTimeoutExceeded bool          `json:"isValidationTimeoutExceeded"`
	IsStandardTx                bool          `json:"isStandardTx"`
	RejectionCode               int           `json:"rejectionCode"`
	Reason                      string        `json:"reason"`
	RejectionReason             string        `json:"rejectionReason"`
	CollidedWith                []CollidingTx `json:"collidedWith"`
	RejectionTime               string        `json:"rejectionTime"`
}

type ZMQDiscardFromMempool struct {
	TxID         string       `json:"txid"`
	Reason       string       `json:"reason"`
	CollidedWith *CollidingTx `json:"collidedWith"`
	BlockHash    string       `json:"blockhash"`
}

type CollidingTx struct {
	TxID string `json:"txid"`
	Hex  string `json:"hex"`
	Size int    `json:"size"`
}

type ZMQI interface {
	Subscribe(string, chan []string) error
}

func NewZMQ(zmqURL *url.URL, statusMessageCh chan<- *TxStatusMessage, zmqHandler ZMQI, logger *slog.Logger) (*ZMQ, error) {
	if zmqHandler == nil {
		return nil, ErrNilZMQHandler
	}

	z := &ZMQ{
		url:             zmqURL,
		statusMessageCh: statusMessageCh,
		handler:         zmqHandler,
		logger:          logger,
	}

	return z, nil
}

func (z *ZMQ) Start() (func(), error) {
	ch := make(chan []string, 100)

	cleanup := func() {
		close(ch)
	}

	const hashtxTopic = "hashtx2"
	const invalidTxTopic = "invalidtx"
	const discardedFromMempoolTopic = "discardedfrommempool"
	go func() {
		for c := range ch {
			switch c[0] {
			case hashtxTopic:
				z.logger.Log(context.Background(), logger.LevelTrace, hashtxTopic, slog.String("hash", c[1]))
				hash, err := chainhash.NewHashFromStr(c[1])
				if err != nil {
					z.logger.Error("failed to get hash from string", slog.String("topic", hashtxTopic), slog.String("err", err.Error()))
					continue
				}

				z.statusMessageCh <- &TxStatusMessage{
					Start:  time.Now(),
					Hash:   hash,
					Status: metamorph_api.Status_ACCEPTED_BY_NETWORK,
					Peer:   z.url.String(),
					Err:    nil,
				}

			case invalidTxTopic:
				hash, status, txErr, competingTxs, err := z.handleInvalidTx(c)
				if err != nil {
					z.logger.Error("failed to parse tx info", slog.String("topic", invalidTxTopic), slog.String("err", err.Error()))
					continue
				}

				if len(competingTxs) == 0 {
					z.statusMessageCh <- &TxStatusMessage{
						Start:        time.Now(),
						Hash:         hash,
						Status:       status,
						Peer:         z.url.String(),
						Err:          txErr,
						CompetingTxs: competingTxs,
					}
					continue
				}

				msgs := z.prepareCompetingTxMsgs(hash, competingTxs)
				for _, msg := range msgs {
					z.statusMessageCh <- msg
				}

			case discardedFromMempoolTopic:
				hash, txErr, err := z.handleDiscardedFromMempool(c)
				if err != nil {
					z.logger.Error("failed to parse", slog.String("topic", discardedFromMempoolTopic), slog.String("err", err.Error()))
					continue
				}

				z.statusMessageCh <- &TxStatusMessage{
					Start:  time.Now(),
					Hash:   hash,
					Status: metamorph_api.Status_REJECTED,
					Peer:   z.url.String(),
					Err:    txErr,
				}
			default:
				z.logger.Info("Unhandled ZMQ message", slog.String("msg", strings.Join(c, ",")))
			}
		}
	}()

	if err := z.handler.Subscribe(hashtxTopic, ch); err != nil {
		return cleanup, err
	}

	if err := z.handler.Subscribe(invalidTxTopic, ch); err != nil {
		return cleanup, err
	}

	if err := z.handler.Subscribe(discardedFromMempoolTopic, ch); err != nil {
		return cleanup, err
	}

	return cleanup, nil
}

func (z *ZMQ) handleInvalidTx(msg []string) (hash *chainhash.Hash, status metamorph_api.Status, txErr error, competingTxs []string, err error) {
	txInfo, err := z.parseTxInfo(msg)
	if err != nil {
		return
	}

	hash, err = chainhash.NewHashFromStr(txInfo.TxID)
	if err != nil {
		return
	}

	if txInfo.IsMissingInputs {
		fmt.Println("shota 1")
		// Missing Inputs does not immediately mean it's an error. It may mean that transaction is temporarily waiting
		// for its parents (e.g. in case of bulk submit). So we don't throw any error here, just update the status.
		// If it's actually an error with transaction, it will be rejected when the parents arrive to node's memopool.
		// If the parents never arrive, it will be discarded from mempool after some time - rejected.
		status = metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL
		return
	}

	if txInfo.IsMempoolConflictDetected || txInfo.IsDoubleSpendDetected || len(txInfo.CollidedWith) > 0 {
		// In case of Double Spend Attempted, we don't want to add an error to the response, as it will immediately
		// return this response to the user. We want to inform him about the status update by callback and wait,
		// because the node can accept one of the transactions from double spend and reject the other(s). Then,
		// we will update the other transactions with the REJECTED status and an error.
		status = metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED
		for _, tx := range txInfo.CollidedWith {
			competingTxs = append(competingTxs, tx.TxID)
			z.logger.Debug("invalidtx", slog.String("hash", txInfo.TxID), slog.String("colliding tx id", tx.TxID), slog.String("colliding tx hex", tx.Hex))
		}
		return
	}

	// If the error is not Missing Inputs, nor Double Spend Detected, then we can assume
	// that the transaction was rejected by the nodes and update the status accordingly.
	status = metamorph_api.Status_REJECTED

	errReason := "invalid transaction"
	if txInfo.RejectionReason != "" {
		errReason += ": " + txInfo.RejectionReason
	}
	z.logger.Debug("invalidtx", slog.String("hash", txInfo.TxID), slog.String("reason", errReason))

	txErr = errors.New(errReason)
	return
}

func (z *ZMQ) parseTxInfo(c []string) (*ZMQTxInfo, error) {
	var txInfo ZMQTxInfo
	txInfoBytes, err := hex.DecodeString(c[1])
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(txInfoBytes, &txInfo)
	if err != nil {
		return nil, err
	}
	return &txInfo, nil
}

func (z *ZMQ) prepareCompetingTxMsgs(hash *chainhash.Hash, competingTxs []string) []*TxStatusMessage {
	msgs := []*TxStatusMessage{{
		Start:        time.Now(),
		Hash:         hash,
		Status:       metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
		Peer:         z.url.String(),
		Err:          nil,
		CompetingTxs: competingTxs,
	}}

	allCompetingTxs := append(competingTxs, hash.String())

	for _, tx := range competingTxs {
		competingHash, err := chainhash.NewHashFromStr(tx)
		if err != nil {
			z.logger.Warn("could not parse competing tx hash",
				slog.String("reportingTxHash", hash.String()),
				slog.String("err", err.Error()),
				slog.String("competingTxID", tx))
			continue
		}

		// remove the hash of the current tx from all competing txs
		// and return a copy of the slice
		txsWithoutSelf := removeCompetingSelf(allCompetingTxs, tx)

		msgs = append(msgs, &TxStatusMessage{
			Start:        time.Now(),
			Hash:         competingHash,
			Status:       metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
			Peer:         z.url.String(),
			Err:          nil,
			CompetingTxs: txsWithoutSelf,
		})
	}

	return msgs
}

func removeCompetingSelf(competingTxs []string, self string) []string {
	copyTxs := make([]string, len(competingTxs))
	copy(copyTxs, competingTxs)

	for i, hash := range copyTxs {
		if hash == self {
			// Remove the self hash
			return append(copyTxs[:i], copyTxs[i+1:]...)
		}
	}

	// Return the slice unchanged if the value was not found
	return copyTxs
}

func (z *ZMQ) handleDiscardedFromMempool(msg []string) (hash *chainhash.Hash, txErr, err error) {
	topic := "discardedfrommempool"
	discardedInfo, err := z.parseDiscardedInfo(msg)
	if err != nil {
		return
	}

	hash, err = chainhash.NewHashFromStr(discardedInfo.TxID)
	if err != nil {
		return
	}

	if discardedInfo.CollidedWith != nil {
		z.logger.Debug(topic, slog.String("hash", discardedInfo.TxID), slog.String("reason", discardedInfo.Reason), slog.String("collidedWith", discardedInfo.CollidedWith.TxID))
		txErr = errors.New("discarded from mempool: double spend attempted")
	} else {
		z.logger.Debug(topic, slog.String("hash", discardedInfo.TxID), slog.String("reason", discardedInfo.Reason))
		txErr = fmt.Errorf("discarded from mempool: %s", discardedInfo.Reason)
	}

	return
}

func (z *ZMQ) parseDiscardedInfo(c []string) (*ZMQDiscardFromMempool, error) {
	var txInfo ZMQDiscardFromMempool
	txInfoBytes, err := hex.DecodeString(c[1])
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(txInfoBytes, &txInfo)
	if err != nil {
		return nil, err
	}
	return &txInfo, nil
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
