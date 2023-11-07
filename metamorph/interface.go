package metamorph

import (
	"context"
	"time"

	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/metamorph/processor_response"
	"github.com/bitcoin-sv/arc/metamorph/store"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

type ProcessorRequest struct {
	*store.StoreData
	ResponseChannel chan processor_response.StatusAndError
	context         context.Context
}

func NewProcessorRequest(ctx context.Context,
	data *store.StoreData,
	responseChannel chan processor_response.StatusAndError) *ProcessorRequest {
	return &ProcessorRequest{
		StoreData:       data,
		ResponseChannel: responseChannel,
		context:         ctx,
	}
}

type ProcessorI interface {
	LoadUnmined()
	Set(req *ProcessorRequest) error
	ProcessTransaction(req *ProcessorRequest)
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
