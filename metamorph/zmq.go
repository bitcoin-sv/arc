package metamorph

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"sync/atomic"

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
	URL       *url.URL
	processor ProcessorI
	logger    *gocore.Logger
	stats     *ZMQStats
}

func NewZMQ(zmqURL *url.URL, processor ProcessorI) *ZMQ {
	var zmqLogger = gocore.Log("zmq")
	z := &ZMQ{
		URL:       zmqURL,
		processor: processor,
		logger:    zmqLogger,
		stats: &ZMQStats{
			hashTx:               atomic.Uint64{},
			invalidTx:            atomic.Uint64{},
			discardedFromMempool: atomic.Uint64{},
		},
	}

	_ = newZMQCollector(z.stats)

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
				z.stats.hashTx.Add(1)
				z.logger.Debugf("hashtx %s", c[1])
				_, _ = z.processor.SendStatusForTransaction(
					c[1],
					metamorph_api.Status_ACCEPTED_BY_NETWORK,
					z.URL.String(),
					nil,
				)
			case "invalidtx":
				z.stats.invalidTx.Add(1)
				// c[1] is lots of info about the tx in json format encoded in hex
				var txInfo map[string]interface{}
				txInfo, err = z.parseTxInfo(c)
				if err != nil {
					z.logger.Error("invalidtx: failed to hex decode tx info")
					continue
				}
				errReason := "invalid transaction"
				if txInfo["reject-reason"] != nil {
					errReason += ": " + txInfo["reject-reason"].(string)
				}
				if txInfo["isMissingInputs"] != nil && txInfo["isMissingInputs"].(bool) {
					errReason += " - missing inputs"
				}
				if txInfo["isDoubleSpend"] != nil && txInfo["isDoubleSpend"].(bool) {
					errReason += " - double spend"
				}
				z.logger.Warnf("invalidtx %s: %s", txInfo["txid"].(string), errReason)
				_, _ = z.processor.SendStatusForTransaction(
					txInfo["txid"].(string),
					metamorph_api.Status_REJECTED,
					z.URL.String(),
					fmt.Errorf(errReason),
				)
			case "discardedfrommempool":
				z.stats.discardedFromMempool.Add(1)
				var txInfo map[string]interface{}
				txInfo, err = z.parseTxInfo(c)
				if err != nil {
					z.logger.Error("invalidtx: failed to hex decode tx info")
					continue
				}
				reason := ""
				if txInfo["reason"] != nil {
					reason = txInfo["reason"].(string)
				}
				// reasons can be "collision-in-block-tx" and "unknown-reason"
				z.logger.Warnf("discardedfrommempool %s: %s", txInfo["txid"].(string), reason)
				_, _ = z.processor.SendStatusForTransaction(
					txInfo["txid"].(string),
					metamorph_api.Status_REJECTED,
					z.URL.String(),
					fmt.Errorf("discarded from mempool: %s", reason),
				)
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

func (z *ZMQ) parseTxInfo(c []string) (map[string]interface{}, error) {
	var txInfo map[string]interface{}
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
