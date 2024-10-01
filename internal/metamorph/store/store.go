package store

import (
	"context"
	"encoding/json"
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
	LastModified      time.Time
	Hash              *chainhash.Hash
	Status            metamorph_api.Status
	BlockHeight       uint64
	BlockHash         *chainhash.Hash
	Callbacks         []StoreCallback
	StatusHistory     []*StoreStatus
	FullStatusUpdates bool
	RejectReason      string
	CompetingTxs      []string
	LockedBy          string
	Ttl               int64
	MerklePath        string
	LastSubmittedAt   time.Time
	Retries           int
}

type StoreCallback struct {
	CallbackURL   string `json:"callback_url"`
	CallbackToken string `json:"callback_token"`
}

type StoreStatus struct {
	Status    metamorph_api.Status
	Timestamp time.Time
}

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
	StatusNotMined             int64
	StatusSeenOnNetworkTotal   int64
	StatusMinedTotal           int64
}

type MetamorphStore interface {
	Get(ctx context.Context, key []byte) (*StoreData, error)
	GetMany(ctx context.Context, keys [][]byte) ([]*StoreData, error)
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
	UpdateDoubleSpend(ctx context.Context, updates []UpdateStatus) ([]*StoreData, error)
	Close(ctx context.Context) error
	ClearData(ctx context.Context, retentionDays int32) (int64, error)
	Ping(ctx context.Context) error

	GetStats(ctx context.Context, since time.Time, notSeenLimit time.Duration, notMinedLimit time.Duration) (*Stats, error)
	GetRawTxs(ctx context.Context, hashes [][]byte) ([][]byte, error)
}

type UpdateStatus struct {
	Hash         chainhash.Hash       `json:"-"`
	Status       metamorph_api.Status `json:"status"`
	Error        error                `json:"-"`
	CompetingTxs []string             `json:"competing_txs"`
	// Fields for marshalling
	hashStr  string
	errorStr string
}

// UnmarshalJSON Custom method to unmarshall the UpdateStatus struct
func (u *UpdateStatus) UnmarshalJSON(data []byte) error {
	type Alias UpdateStatus
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(u),
	}

	// Unmarshal into the temporary struct
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Convert the error string back to an error if necessary
	if u.errorStr != "" {
		u.Error = errors.New(u.errorStr)
	}

	// Convert the hash string back to a chainhash.Hash
	hash, err := chainhash.NewHashFromStr(u.hashStr)
	if err != nil {
		return err
	}
	u.Hash = *hash

	return nil
}

// MarshalJSON Custom method to marshall the UpdateStatus struct
func (u UpdateStatus) MarshalJSON() ([]byte, error) {
	type Alias UpdateStatus
	if u.Error != nil {
		u.errorStr = u.Error.Error() // Convert error to string for marshaling
	}

	u.hashStr = u.Hash.String() // Convert hash to string for marshaling

	return json.Marshal((*Alias)(&u))
}
