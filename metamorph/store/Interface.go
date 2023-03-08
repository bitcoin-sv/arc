package store

import (
	"context"
	"errors"
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

type StoreData struct {
	StoredAt      time.Time
	AnnouncedAt   time.Time
	MinedAt       time.Time
	Hash          *chainhash.Hash `badgerhold:"key"`
	Status        metamorph_api.Status
	BlockHeight   uint64
	BlockHash     *chainhash.Hash
	ApiKeyId      int64
	StandardFeeId int64
	DataFeeId     int64
	SourceIp      string
	CallbackUrl   string
	CallbackToken string
	MerkleProof   bool
	RawTx         []byte
	RejectReason  string
}

var ErrNotFound = errors.New("txid could not be found")

type MetamorphStore interface {
	Get(ctx context.Context, key []byte) (*StoreData, error)
	Set(ctx context.Context, key []byte, value *StoreData) error
	Del(ctx context.Context, key []byte) error

	GetUnmined(_ context.Context, callback func(s *StoreData)) error
	UpdateStatus(ctx context.Context, hash *chainhash.Hash, status metamorph_api.Status, rejectReason string) error
	UpdateMined(ctx context.Context, hash *chainhash.Hash, blockHash *chainhash.Hash, blockHeight uint64) error
	Close(ctx context.Context) error
	GetBlockProcessed(ctx context.Context, blockHash *chainhash.Hash) (*time.Time, error)
	SetBlockProcessed(ctx context.Context, blockHash *chainhash.Hash) error
}
