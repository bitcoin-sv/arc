package metamorph

import (
	"context"
	"time"

	"github.com/libsv/go-p2p/chaincfg/chainhash"

	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/metamorph/processor_response"
	"github.com/bitcoin-sv/arc/metamorph/store"
)

type ProcessorRequest struct {
	Data            *store.StoreData
	ResponseChannel chan processor_response.StatusAndError
}

type ProcessorI interface {
	LoadUnmined()
	Set(ctx context.Context, req *ProcessorRequest) error
	ProcessTransaction(ctx context.Context, req *ProcessorRequest)
	SendStatusForTransaction(hash *chainhash.Hash, status metamorph_api.Status, id string, err error) (bool, error)
	SendStatusMinedForTransaction(hash *chainhash.Hash, blockHash *chainhash.Hash, blockHeight uint64) (bool, error)
	GetStats(debugItems bool) *ProcessorStats
	GetPeers() ([]string, []string)
	Shutdown()
}

type PeerTxMessage struct {
	Start  time.Time
	Hash   *chainhash.Hash
	Status metamorph_api.Status
	Peer   string
	Err    error
}
