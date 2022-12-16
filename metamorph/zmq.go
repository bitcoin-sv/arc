package metamorph

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	pb "github.com/TAAL-GmbH/arc/metamorph/api"
	"github.com/ordishs/go-bitcoin"
	"github.com/ordishs/gocore"
)

type ZMQ struct {
	processor *Processor
	logger    *gocore.Logger
}

func NewZMQ(processor *Processor) *ZMQ {
	var zmqLogger = gocore.Log("zmq")
	return &ZMQ{
		processor: processor,
		logger:    zmqLogger,
	}
}

func (z *ZMQ) Start() {
	zmqHost, _ := gocore.Config().Get("peer_1_host", "localhost")
	zmqPort, _ := gocore.Config().GetInt("peer_1_zmqPort", 28332)

	z.logger.Infof("Listening to ZMQ on %s:%d", zmqHost, zmqPort)

	zmq := bitcoin.NewZMQ(zmqHost, zmqPort, z.logger)

	ch := make(chan []string)

	go func() {
		for c := range ch {
			switch c[0] {
			case "hashtx":
				z.processor.SendStatusForTransaction(c[1], pb.Status_SEEN_ON_NETWORK, nil)
			case "invalidtx":
				// c[1] is lots of info about the tx in json format encoded in hex
				txInfo, err := z.parseTxInfo(c)
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
				z.processor.SendStatusForTransaction(txInfo["txid"].(string), pb.Status_REJECTED, fmt.Errorf(errReason))
			case "discardedfrommempool":
				txInfo, err := z.parseTxInfo(c)
				if err != nil {
					z.logger.Error("invalidtx: failed to hex decode tx info")
					continue
				}
				reason := ""
				if txInfo["reason"] != nil {
					reason = txInfo["reason"].(string)
				}
				// reasons can be "collision-in-block-tx" and "unknown-reason"
				z.processor.SendStatusForTransaction(txInfo["txid"].(string), pb.Status_REJECTED, fmt.Errorf("discarded from mempool: %s", reason))
			case "removedfrommempoolblock":
				txInfo, err := z.parseTxInfo(c)
				if err != nil {
					z.logger.Error("invalidtx: failed to hex decode tx info")
					continue
				}
				// TODO other reasons are "reorg" and "unknown-reason" - what to do then?
				if txInfo["reason"] != nil && txInfo["reason"].(string) == "included-in-block" {
					z.processor.SendStatusForTransaction(txInfo["txid"].(string), pb.Status_MINED, nil)
				} else {
					z.logger.Error("removedfrommempoolblock: unable to handle reason", txInfo["reason"])
				}
			default:
				z.logger.Info("Unhandled ZMQ message", c)
			}
		}
	}()

	if err := zmq.Subscribe("hashtx", ch); err != nil {
		z.logger.Fatal(err)
	}

	if err := zmq.Subscribe("hashblock", ch); err != nil {
		z.logger.Fatal(err)
	}

	if err := zmq.Subscribe("invalidtx", ch); err != nil {
		z.logger.Fatal(err)
	}

	if err := zmq.Subscribe("discardedfrommempool", ch); err != nil {
		z.logger.Fatal(err)
	}

	if err := zmq.Subscribe("removedfrommempoolblock", ch); err != nil {
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
