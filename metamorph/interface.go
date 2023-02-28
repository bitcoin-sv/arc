package metamorph

import (
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/libsv/go-p2p"
)

type ProcessorI interface {
	LoadUnmined()
	ProcessTransaction(req *ProcessorRequest)
	SendStatusForTransaction(hashStr string, status metamorph_api.Status, id string, err error) (bool, error)
	SendStatusMinedForTransaction(hash []byte, blockHash []byte, blockHeight uint64) (bool, error)
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
