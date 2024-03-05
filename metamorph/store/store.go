package store

import (
	"context"
	"errors"
	"time"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

var ErrNotFound = errors.New("key could not be found")

type StoreData struct {
	RawTx             []byte               `dynamodbav:"raw_tx"`
	StoredAt          time.Time            `dynamodbav:"stored_at"`
	AnnouncedAt       time.Time            `dynamodbav:"announced_at"`
	MinedAt           time.Time            `dynamodbav:"mined_at"`
	Hash              *chainhash.Hash      `badgerhold:"key"            dynamodbav:"tx_hash"`
	Status            metamorph_api.Status `dynamodbav:"tx_status"`
	BlockHeight       uint64               `dynamodbav:"block_height"`
	BlockHash         *chainhash.Hash      `dynamodbav:"block_hash"`
	CallbackUrl       string               `dynamodbav:"callback_url"`
	FullStatusUpdates bool                 `dynamodbav:"full_status_updates"`
	CallbackToken     string               `dynamodbav:"callback_token"`
	RejectReason      string               `dynamodbav:"reject_reason"`
	LockedBy          string               `dynamodbav:"locked_by"`
	Ttl               int64                `dynamodbav:"ttl"`
	MerklePath        string               `dynamodbav:"merkle_path"`
	InsertedAtNum     int                  `dynamodbav:"inserted_at_num"`
}

type MetamorphStore interface {
	Get(ctx context.Context, key []byte) (*StoreData, error)
	Set(ctx context.Context, key []byte, value *StoreData) error
	Del(ctx context.Context, key []byte) error

	SetUnlocked(ctx context.Context, hashes []*chainhash.Hash) error
	SetUnlockedByName(ctx context.Context, lockedBy string) (int64, error)
	GetUnmined(ctx context.Context, since time.Time, limit int64) ([]*StoreData, error)
	UpdateStatusBulk(ctx context.Context, updates []UpdateStatus) ([]*StoreData, error)
	UpdateMined(ctx context.Context, txsBlocks *blocktx_api.TransactionBlocks) ([]*StoreData, error)
	Close(ctx context.Context) error
	ClearData(ctx context.Context, retentionDays int32) (int64, error)
	Ping(ctx context.Context) error
}

type UpdateStatus struct {
	Hash         chainhash.Hash
	Status       metamorph_api.Status
	RejectReason string
}
