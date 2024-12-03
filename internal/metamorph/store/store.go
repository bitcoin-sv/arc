package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/libsv/go-p2p/chaincfg/chainhash"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
)

var ErrNotFound = errors.New("key could not be found")

type Data struct {
	RawTx             []byte
	StoredAt          time.Time
	LastModified      time.Time
	Hash              *chainhash.Hash
	Status            metamorph_api.Status
	BlockHeight       uint64
	BlockHash         *chainhash.Hash
	Callbacks         []Callback
	StatusHistory     []*Status
	FullStatusUpdates bool
	RejectReason      string
	CompetingTxs      []string
	LockedBy          string
	TTL               int64
	MerklePath        string
	LastSubmittedAt   time.Time
	Retries           int
}

type Callback struct {
	CallbackURL   string `json:"callback_url"`
	CallbackToken string `json:"callback_token"`
	AllowBatch    bool   `json:"allow_batch"`
}

type Status struct {
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
	StatusNotFinal             int64
	StatusSeenOnNetworkTotal   int64
	StatusMinedTotal           int64
}

type MetamorphStore interface {
	Get(ctx context.Context, key []byte) (*Data, error)
	GetMany(ctx context.Context, keys [][]byte) ([]*Data, error)
	Set(ctx context.Context, value *Data) error
	SetBulk(ctx context.Context, data []*Data) error
	Del(ctx context.Context, key []byte) error

	SetLocked(ctx context.Context, since time.Time, limit int64) error
	IncrementRetries(ctx context.Context, hash *chainhash.Hash) error
	SetUnlockedByName(ctx context.Context, lockedBy string) (int64, error)
	GetUnmined(ctx context.Context, since time.Time, limit int64, offset int64) ([]*Data, error)
	GetSeenOnNetwork(ctx context.Context, since time.Time, until time.Time, limit int64, offset int64) ([]*Data, error)
	UpdateStatusBulk(ctx context.Context, updates []UpdateStatus) ([]*Data, error)
	UpdateMined(ctx context.Context, txsBlocks []*blocktx_api.TransactionBlock) ([]*Data, error)
	UpdateDoubleSpend(ctx context.Context, updates []UpdateStatus) ([]*Data, error)
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
	HashStr  string `json:"hash"`
	ErrorStr string `json:"error"`
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
	if u.ErrorStr != "" {
		u.Error = errors.New(u.ErrorStr)
	}

	// Convert the hash string back to a chainhash.Hash
	hash, err := chainhash.NewHashFromStr(u.HashStr)
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
		u.ErrorStr = u.Error.Error() // Convert error to string for marshaling
	}

	u.HashStr = u.Hash.String() // Convert hash to string for marshaling

	return json.Marshal((*Alias)(&u))
}
