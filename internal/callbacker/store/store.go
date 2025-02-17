package store

import (
	"context"
	"errors"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/go-sdk/chainhash"
	"time"
)

var (
	ErrURLMappingDuplicateKey  = errors.New("URL mapping duplicate key")
	ErrURLMappingDeleteFailed  = errors.New("failed to delete URL mapping entry")
	ErrURLMappingsDeleteFailed = errors.New("failed to delete URL mapping entries")
	ErrNoUnmappedURLsFound     = errors.New("no unmapped URLs found")
)

type Stats struct {
	StatusStored               int64
	StatusAnnouncedToNetwork   int64
	StatusRequestedByNetwork   int64
	StatusSentToNetwork        int64
	StatusAcceptedByNetwork    int64
	StatusSeenInOrphanMempool  int64
	StatusSeenOnNetwork        int64
	StatusDoubleSpendAttempted int64
	StatusRejected             int64
	StatusMined                int64
	StatusNotSeen              int64
	StatusNotFinal             int64
	StatusSeenOnNetworkTotal   int64
	StatusMinedTotal           int64
}

type UpdateStatus struct {
	Hash          chainhash.Hash        `json:"-"`
	Status        metamorph_api.Status  `json:"status"`
	Error         error                 `json:"-"`
	CompetingTxs  []string              `json:"competing_txs"`
	StatusHistory []StatusWithTimestamp `json:"status_history"`
	Timestamp     time.Time             `json:"timestamp"`
	// Fields for marshalling
	HashStr  string `json:"hash"`
	ErrorStr string `json:"error"`
}

type CallbackData struct {
	URL   string
	Token string

	Timestamp time.Time

	CompetingTxs []string

	TxID       string
	TxStatus   string
	ExtraInfo  *string
	MerklePath *string

	BlockHash   *string
	BlockHeight *uint64

	AllowBatch bool
}

type ProcessorStore interface {
	SetURLMapping(ctx context.Context, m URLMapping) error
	GetURLMappings(ctx context.Context) (urlInstanceMappings map[string]string, err error)
	DeleteURLMapping(ctx context.Context, instance string) (rowsAffected int64, err error)
	GetUnmappedURL(ctx context.Context) (url string, err error)
	GetAndDelete(ctx context.Context, url string, limit int) ([]*CallbackData, error)
	DeleteOlderThan(ctx context.Context, t time.Time) error
}

type CallbackStore interface {
	DeleteURLMappingsExcept(ctx context.Context, except []string) (rowsAffected int64, err error)
}

type URLMapping struct {
	URL      string
	Instance string
}

type StatusWithTimestamp struct {
	Status    metamorph_api.Status `json:"status"`
	Timestamp time.Time            `json:"timestamp"`
}
