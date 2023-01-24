package metamorph

import (
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/p2p"
)

type ProcessorI interface {
	LoadUnseen()
	ProcessTransaction(req *ProcessorRequest)
	SendStatusForTransaction(hashStr string, status metamorph_api.Status, err error) (bool, error)
	SendStatusMinedForTransaction(hash []byte, blockHash []byte, blockHeight int32) (bool, error)
	GetStats() *ProcessorStats
	GetPeers() ([]string, []string)
}

type PeerTxMessage struct {
	Start  time.Time
	Txid   string
	Status p2p.Status
	Peer   string
	Err    error
}
