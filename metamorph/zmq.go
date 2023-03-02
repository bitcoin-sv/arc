package metamorph

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/ordishs/go-bitcoin"
	"github.com/ordishs/gocore"
)

type ZMQStats struct {
	hashTx               atomic.Uint64
	invalidTx            atomic.Uint64
	discardedFromMempool atomic.Uint64
}

type ZMQ struct {
	URL             *url.URL
	Stats           *ZMQStats
	statusMessageCh chan<- *PeerTxMessage
	logger          *gocore.Logger
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
	RejectionCode               bool          `json:"rejectionCode"`
	Reason                      string        `json:"reason"`
	RejectionReason             string        `json:"rejectionReason"`
	CollidedWith                []interface{} `json:"collidedWith"`
	RejectionTime               string        `json:"rejectionTime"`
}

func NewZMQ(zmqURL *url.URL, statusMessageCh chan<- *PeerTxMessage) *ZMQ {
	var zmqLogger = gocore.Log("zmq")
	z := &ZMQ{
		URL: zmqURL,
		Stats: &ZMQStats{
			hashTx:               atomic.Uint64{},
			invalidTx:            atomic.Uint64{},
			discardedFromMempool: atomic.Uint64{},
		},
		statusMessageCh: statusMessageCh,
		logger:          zmqLogger,
	}

	return z
}

func (z *ZMQ) Start() {
	port, err := strconv.Atoi(z.URL.Port())
	if err != nil {
		z.logger.Fatalf("Could not parse port from metamorph_zmqAddress: %v", err)
	}

	z.logger.Infof("Listening to ZMQ on %s:%d", z.URL.Hostname(), port)

	zmq := bitcoin.NewZMQ(z.URL.Hostname(), port, z.logger)

	ch := make(chan []string)

	go func() {
		for c := range ch {
			switch c[0] {
			case "hashtx":
				z.Stats.hashTx.Add(1)
				z.logger.Debugf("hashtx %s", c[1])
				z.statusMessageCh <- &PeerTxMessage{
					Start:  time.Now(),
					Txid:   c[1],
					Status: metamorph_api.Status_ACCEPTED_BY_NETWORK,
					Peer:   z.URL.String(),
				}
			case "invalidtx":
				z.Stats.invalidTx.Add(1)
				// c[1] is lots of info about the tx in json format encoded in hex
				var txInfo *ZMQTxInfo
				txInfo, err = z.parseTxInfo(c)
				if err != nil {
					z.logger.Error("invalidtx: failed to hex decode tx info")
					continue
				}
				errReason := "invalid transaction"
				if txInfo.RejectionReason != "" {
					errReason += ": " + txInfo.RejectionReason
				}
				if txInfo.IsMissingInputs {
					errReason += " - missing inputs"
				}
				if txInfo.IsDoubleSpendDetected {
					errReason += " - double spend"
				}
				z.logger.Debugf("invalidtx %s: %s", txInfo.TxID, errReason)
				z.statusMessageCh <- &PeerTxMessage{
					Start:  time.Now(),
					Txid:   txInfo.TxID,
					Status: metamorph_api.Status_REJECTED,
					Peer:   z.URL.String(),
					Err:    fmt.Errorf(errReason),
				}
			case "discardedfrommempool":
				z.Stats.discardedFromMempool.Add(1)
				var txInfo *ZMQTxInfo
				txInfo, err = z.parseTxInfo(c)
				if err != nil {
					z.logger.Error("invalidtx: failed to hex decode tx info")
					continue
				}
				reason := ""
				if txInfo.Reason != "" {
					reason = txInfo.Reason
				}
				if txInfo.RejectionReason != "" {
					reason += ": " + txInfo.RejectionReason
				}
				// reasons can be "collision-in-block-tx" and "unknown-reason"
				z.logger.Debugf("discardedfrommempool %s: %s", txInfo.TxID, reason)
				z.statusMessageCh <- &PeerTxMessage{
					Start:  time.Now(),
					Txid:   txInfo.TxID,
					Status: metamorph_api.Status_REJECTED,
					Peer:   z.URL.String(),
					Err:    fmt.Errorf("discarded from mempool: %s", reason),
				}
			default:
				z.logger.Info("Unhandled ZMQ message", c)
			}
		}
	}()

	if err = zmq.Subscribe("hashtx", ch); err != nil {
		z.logger.Fatal(err)
	}

	if err = zmq.Subscribe("invalidtx", ch); err != nil {
		z.logger.Fatal(err)
	}

	if err = zmq.Subscribe("discardedfrommempool", ch); err != nil {
		z.logger.Fatal(err)
	}
}

func (z *ZMQ) parseTxInfo(c []string) (*ZMQTxInfo, error) {
	var txInfo *ZMQTxInfo
	txInfoBytes, err := hex.DecodeString(c[1])
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(txInfoBytes, &txInfo)
	if err != nil {
		return nil, err
	}
	return txInfo, nil
}
