package metamorph

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bitcoin-sv/arc/pkg/metamorph/metamorph_api"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

type ZMQStats struct {
	hashTx               atomic.Uint64
	invalidTx            atomic.Uint64
	discardedFromMempool atomic.Uint64
}

type ZMQ struct {
	url             *url.URL
	stats           *ZMQStats
	statusMessageCh chan<- *PeerTxMessage
	logger          *slog.Logger
}

func (z *ZMQ) GetStats() *ZMQStats {
	return z.stats
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
	CollidedWith                []interface{} `json:"collidedWith"`
	RejectionTime               string        `json:"rejectionTime"`
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
		url: zmqURL,
		stats: &ZMQStats{
			hashTx:               atomic.Uint64{},
			invalidTx:            atomic.Uint64{},
			discardedFromMempool: atomic.Uint64{},
		},
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
		defer func() {
			if r := recover(); r != nil {
				z.logger.Error("Recovered from panic", slog.String("stacktrace", string(debug.Stack())))
			}
		}()
		var err error
		for c := range ch {
			switch c[0] {
			case hashtxTopic:
				z.stats.hashTx.Add(1)
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
				z.stats.invalidTx.Add(1)
				// c[1] is lots of info about the tx in json format encoded in hex
				var txInfo *ZMQTxInfo
				status := metamorph_api.Status_REJECTED
				txInfo, err = z.parseTxInfo(c)
				if err != nil {
					z.logger.Error("failed to parse tx info", slog.String("topic", invalidTxTopic), slog.String("err", err.Error()))
					continue
				}
				errReason := "invalid transaction"
				if txInfo.RejectionReason != "" {
					errReason += ": " + txInfo.RejectionReason
				}
				if txInfo.IsMissingInputs {
					errReason = ""
					status = metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL
				}
				if txInfo.IsDoubleSpendDetected {
					errReason += " - double spend"
				}

				z.logger.Debug(invalidTxTopic, slog.String("hash", txInfo.TxID), slog.String("reason", errReason))

				hash, err := chainhash.NewHashFromStr(txInfo.TxID)
				if err != nil {
					z.logger.Error("failed to get hash from string", slog.String("topic", invalidTxTopic), slog.String("err", err.Error()))
					continue
				}

				z.statusMessageCh <- &PeerTxMessage{
					Start:  time.Now(),
					Hash:   hash,
					Status: status,
					Peer:   z.url.String(),
					Err:    fmt.Errorf(errReason),
				}
			case discardedFromMempoolTopic:
				z.stats.discardedFromMempool.Add(1)
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
