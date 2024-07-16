package store

import (
	"context"
	"errors"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

var ErrNotFound = errors.New("key could not be found")

type StoreData struct {
	RawTx             []byte
	StoredAt          time.Time
	AnnouncedAt       time.Time
	MinedAt           time.Time
	Hash              *chainhash.Hash
	Status            metamorph_api.Status
	BlockHeight       uint64
	BlockHash         *chainhash.Hash
	CallbackUrl       string
	FullStatusUpdates bool
	CallbackToken     string
	RejectReason      string
	LockedBy          string
	Ttl               int64
	MerklePath        string
	LastSubmittedAt   time.Time
	Retries           int
}

type Stats struct {
	StatusStored              int64
	StatusAnnouncedToNetwork  int64
	StatusRequestedByNetwork  int64
	StatusSentToNetwork       int64
	StatusAcceptedByNetwork   int64
	StatusSeenOnNetwork       int64
	StatusMined               int64
	StatusRejected            int64
	StatusSeenInOrphanMempool int64
	StatusNotSeen             int64
	StatusNotMined            int64
	StatusSeenOnNetworkTotal  int64
	StatusMinedTotal          int64
}

type MetamorphStore interface {
	Get(ctx context.Context, key []byte) (*StoreData, error)
	Set(ctx context.Context, value *StoreData) error
	SetBulk(ctx context.Context, data []*StoreData) error
	Del(ctx context.Context, key []byte) error

	SetLocked(ctx context.Context, since time.Time, limit int64) error
	IncrementRetries(ctx context.Context, hash *chainhash.Hash) error
	SetUnlockedByName(ctx context.Context, lockedBy string) (int64, error)
	GetUnmined(ctx context.Context, since time.Time, limit int64, offset int64) ([]*StoreData, error)
	GetSeenOnNetwork(ctx context.Context, since time.Time, until time.Time, limit int64, offset int64) ([]*StoreData, error)
	UpdateStatusBulk(ctx context.Context, updates []UpdateStatus) ([]*StoreData, error)
	UpdateMined(ctx context.Context, txsBlocks []*blocktx_api.TransactionBlock) ([]*StoreData, error)
	Close(ctx context.Context) error
	ClearData(ctx context.Context, retentionDays int32) (int64, error)
	Ping(ctx context.Context) error

	GetStats(ctx context.Context, since time.Time, notSeenLimit time.Duration, notMinedLimit time.Duration) (*Stats, error)
	GetRawTxs(ctx context.Context, hashes [][]byte) ([][]byte, error)
}

type UpdateStatus struct {
	Hash         chainhash.Hash
	Status       metamorph_api.Status
	RejectReason string
	CompetingTxs []string
}
