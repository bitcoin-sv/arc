package metamorph

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

type ZMQ struct {
	url             *url.URL
	statusMessageCh chan<- *PeerTxMessage
	logger          *slog.Logger
}

type ZMQTxInfo struct {
	TxID                        string        `json:"txid"`
	FromBlock                   bool          `json:"fromBlock"`
	Source                      string        `json:"source"`
	Address                     string        `json:"address"`
	NodeId                      int           `json:"nodeId"`
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

type CollidingTx struct {
	Hex  string `json:"hex"`
	Size int    `json:"size"`
	TxID string `json:"txid"`
}

type ZMQDiscardFromMempool struct {
	TxID         string `json:"txid"`
	Reason       string `json:"reason"`
	CollidedWith struct {
		TxID string `json:"txid"`
		Size int    `json:"size"`
		Hex  string `json:"hex"`
	} `json:"collidedWith"`
	BlockHash string `json:"blockhash"`
}

func NewZMQ(zmqURL *url.URL, statusMessageCh chan<- *PeerTxMessage, logger *slog.Logger) *ZMQ {
	z := &ZMQ{
		url:             zmqURL,
		statusMessageCh: statusMessageCh,
		logger:          logger,
	}

	return z
}

type ZMQI interface {
	Subscribe(string, chan []string) error
}

func (z *ZMQ) Start(zmqi ZMQI) error {
	ch := make(chan []string)

	const hashtxTopic = "hashtx2"
	const invalidTxTopic = "invalidtx"
	const discardedFromMempoolTopic = "discardedfrommempool"
	go func() {
		var err error
		for c := range ch {
			switch c[0] {
			case hashtxTopic:
				z.logger.Debug(hashtxTopic, slog.String("hash", c[1]))

				hash, err := chainhash.NewHashFromStr(c[1])
				if err != nil {
					z.logger.Error("failed to get hash from string", slog.String("topic", hashtxTopic), slog.String("err", err.Error()))
					continue
				}

				z.statusMessageCh <- &PeerTxMessage{
					Start:  time.Now(),
					Hash:   hash,
					Status: metamorph_api.Status_ACCEPTED_BY_NETWORK,
					Peer:   z.url.String(),
				}
			case invalidTxTopic:
				hash, status, txErr, competingTxs, err := z.handleInvalidTx(c)
				if err != nil {
					z.logger.Error("failed to parse tx info", slog.String("topic", invalidTxTopic), slog.String("err", err.Error()))
					continue
				}

				z.statusMessageCh <- &PeerTxMessage{
					Start:        time.Now(),
					Hash:         hash,
					Status:       status,
					Peer:         z.url.String(),
					Err:          txErr,
					CompetingTxs: competingTxs,
				}
			case discardedFromMempoolTopic:
				var txInfo *ZMQDiscardFromMempool
				txInfo, err = z.parseDiscardedInfo(c)
				if err != nil {
					z.logger.Error("failed to parse", slog.String("topic", discardedFromMempoolTopic), slog.String("err", err.Error()))
					continue
				}

				z.logger.Debug(discardedFromMempoolTopic, slog.String("hash", txInfo.TxID), slog.String("reason", txInfo.Reason), slog.String("collidedWith", txInfo.CollidedWith.TxID))
				hash, err := chainhash.NewHashFromStr(txInfo.TxID)
				if err != nil {
					z.logger.Error("failed to get hash from string", slog.String("topic", discardedFromMempoolTopic), slog.String("err", err.Error()))
					continue
				}

				z.statusMessageCh <- &PeerTxMessage{
					Start:  time.Now(),
					Hash:   hash,
					Status: metamorph_api.Status_REJECTED,
					Peer:   z.url.String(),
					Err:    fmt.Errorf("discarded from mempool: %s", txInfo.Reason),
				}
			default:
				z.logger.Info("Unhandled ZMQ message", slog.String("msg", strings.Join(c, ",")))
			}
		}
	}()

	if err := zmqi.Subscribe(hashtxTopic, ch); err != nil {
		return err
	}

	if err := zmqi.Subscribe(invalidTxTopic, ch); err != nil {
		return err
	}

	if err := zmqi.Subscribe(discardedFromMempoolTopic, ch); err != nil {
		return err
	}

	return nil
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
		status = metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL
		return
	}

	errReason := "invalid transaction"

	if txInfo.IsMempoolConflictDetected || txInfo.IsDoubleSpendDetected || len(txInfo.CollidedWith) > 0 {
		status = metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED
		// TODO: we probably shouldn't return an error here, as this tx can still be mined
		errReason += ": double spend detected"
		for _, ctx := range txInfo.CollidedWith {
			competingTxs = append(competingTxs, ctx.TxID)
			z.logger.Debug("invalidtx", slog.String("hash", txInfo.TxID), slog.String("colliding tx id", ctx.TxID), slog.String("colliding tx hex", ctx.Hex))
		}
	} else {
		status = metamorph_api.Status_REJECTED
		if txInfo.RejectionReason != "" {
			errReason += ": " + txInfo.RejectionReason
		}
	}

	z.logger.Debug("invalidtx", slog.String("hash", txInfo.TxID), slog.String("reason", errReason))
	txErr = fmt.Errorf(errReason)
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
