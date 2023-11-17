package metamorph

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
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
	Logger          *gocore.Logger
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
		Logger:          zmqLogger,
	}

	return z
}

type ZMQI interface {
	Subscribe(string, chan []string) error
}

func (z *ZMQ) Start(zmqi ZMQI) {

	ch := make(chan []string)

	go func() {
		var err error
		for c := range ch {
			switch c[0] {
			case "hashtx2":
				z.Stats.hashTx.Add(1)
				z.Logger.Debugf("hashtx %s", c[1])

				hash, _ := chainhash.NewHashFromStr(c[1])

				z.statusMessageCh <- &PeerTxMessage{
					Start:  time.Now(),
					Hash:   hash,
					Status: metamorph_api.Status_ACCEPTED_BY_NETWORK,
					Peer:   z.URL.String(),
				}
			case "invalidtx":
				z.Stats.invalidTx.Add(1)
				// c[1] is lots of info about the tx in json format encoded in hex
				var txInfo *ZMQTxInfo
				status := metamorph_api.Status_REJECTED
				txInfo, err = z.parseTxInfo(c)
				if err != nil {
					z.Logger.Errorf("invalidtx: failed to parse: %v", err)
					continue
				}
				errReason := "invalid transaction"
				if txInfo.RejectionReason != "" {
					errReason += ": " + txInfo.RejectionReason
				}
				if txInfo.IsMissingInputs {
					z.Logger.Debugf("invalidtx %s due to missing inputs ignored", txInfo.TxID)
					errReason = "Transaction " + txInfo.TxID + " is currently orphaned"
					status = metamorph_api.Status_SENT_TO_NETWORK
				}
				if txInfo.IsDoubleSpendDetected {
					errReason += " - double spend"
				}

				z.Logger.Debugf("invalidtx %s: %s", txInfo.TxID, errReason)

				hash, _ := chainhash.NewHashFromStr(txInfo.TxID)
				z.statusMessageCh <- &PeerTxMessage{
					Start:  time.Now(),
					Hash:   hash,
					Status: status,
					Peer:   z.URL.String(),
					Err:    fmt.Errorf(errReason),
				}
			case "discardedfrommempool":
				z.Stats.discardedFromMempool.Add(1)
				var txInfo *ZMQDiscardFromMempool
				txInfo, err = z.parseDiscardedInfo(c)
				if err != nil {
					z.Logger.Errorf("discardedfrommempool: failed to parse: %v", err)
					continue
				}

				z.Logger.Debugf("discardedfrommempool %s: %s - %#v", txInfo.TxID, txInfo.Reason, txInfo.CollidedWith)

				hash, _ := chainhash.NewHashFromStr(txInfo.TxID)

				z.statusMessageCh <- &PeerTxMessage{
					Start:  time.Now(),
					Hash:   hash,
					Status: metamorph_api.Status_REJECTED,
					Peer:   z.URL.String(),
					Err:    fmt.Errorf("discarded from mempool: %s", txInfo.Reason),
				}
			default:
				z.Logger.Info("Unhandled ZMQ message", c)
			}
		}
	}()

	if err := zmqi.Subscribe("hashtx2", ch); err != nil {
		z.Logger.Fatal(err)
	}

	if err := zmqi.Subscribe("invalidtx", ch); err != nil {
		z.Logger.Fatal(err)
	}

	if err := zmqi.Subscribe("discardedfrommempool", ch); err != nil {
		z.Logger.Fatal(err)
	}
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
