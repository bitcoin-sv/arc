package store

import (
	"context"
	"errors"
	"time"

	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

type StoreData struct {
	RawTx         []byte               `dynamodbav:"raw_tx"`
	StoredAt      time.Time            `dynamodbav:"stored_at"`
	AnnouncedAt   time.Time            `dynamodbav:"announced_at"`
	MinedAt       time.Time            `dynamodbav:"mined_at"`
	Hash          *chainhash.Hash      `badgerhold:"key" dynamodbav:"tx_hash"`
	Status        metamorph_api.Status `dynamodbav:"tx_status"`
	BlockHeight   uint64               `dynamodbav:"block_height"`
	BlockHash     *chainhash.Hash      `dynamodbav:"block_hash"`
	MerkleProof   bool                 `dynamodbav:"merkle_proof"`
	CallbackUrl   string               `dynamodbav:"callback_url"`
	CallbackToken string               `dynamodbav:"callback_token"`
	RejectReason  string               `dynamodbav:"reject_reason"`
}

var ErrNotFound = errors.New("txid could not be found")

type MetamorphStore interface {
	Get(ctx context.Context, key []byte) (*StoreData, error)
	Set(ctx context.Context, key []byte, value *StoreData) error
	Del(ctx context.Context, key []byte) error

	IsCentralised() bool
	GetUnmined(_ context.Context, callback func(s *StoreData)) error
	UpdateStatus(ctx context.Context, hash *chainhash.Hash, status metamorph_api.Status, rejectReason string) error
	UpdateMined(ctx context.Context, hash *chainhash.Hash, blockHash *chainhash.Hash, blockHeight uint64) error
	Close(ctx context.Context) error
	GetBlockProcessed(ctx context.Context, blockHash *chainhash.Hash) (*time.Time, error)
	SetBlockProcessed(ctx context.Context, blockHash *chainhash.Hash) error
}
