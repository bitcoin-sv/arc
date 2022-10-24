package models

import (
	"context"
	"time"

	"github.com/libsv/go-bt/v2"
	"github.com/mrz1836/go-datastore"
	"github.com/taal/mapi"
)

// ModelNameTransaction defines the model name
var ModelNameTransaction ModelName = "transaction"
var TableNameTransaction ModelName = "transactions"

type TransactionStatus string

const (
	TransactionStatusNew     TransactionStatus = "new"
	TransactionStatusValid   TransactionStatus = "valid"
	TransactionStatusMempool TransactionStatus = "mempool"
	TransactionStatusMined   TransactionStatus = "mined"
	TransactionStatusError   TransactionStatus = "error"
)

// Transaction defines the database model for transactions
type Transaction struct {
	Model `bson:",inline"`

	ID            string            `json:"id" toml:"id" yaml:"id" gorm:"<-:create;type:char(64);primaryKey;comment:This is the unique hash of the fee" bson:"_id"`
	BlockHash     string            `json:"block_hash" toml:"block_hash" yaml:"block_hash" gorm:"<-;type:char(64);comment:This is the related block when the transaction was mined" bson:"block_hash,omitempty"`
	BlockHeight   uint64            `json:"block_height" toml:"block_height" yaml:"block_height" gorm:"<-;type:bigint;comment:This is the related block when the transaction was mined" bson:"block_height,omitempty"`
	Tx            []byte            `json:"tx" toml:"tx" yaml:"tx" gorm:"<-:create;type:blob;comment:This is the raw transaction in binary" bson:"tx"`
	Status        TransactionStatus `json:"status" toml:"status" yaml:"status" gorm:"<-type:varchar(7);comment:Internal status of the transaction" bson:"status"`
	ClientID      string            `json:"client_id,omitempty" toml:"client_id" yaml:"client_id" gorm:"<-:create;comment:ClientID for this record" bson:"client_id"`
	CallbackURL   string            `json:"callback_url,omitempty" toml:"callback_url" yaml:"callback_url" gorm:"<-:create;type:text;comment:Callback URL" bson:"callback_url,omitempty"`
	CallbackToken string            `json:"callback_token,omitempty" toml:"callback_token" yaml:"callback_token" gorm:"<-:create;comment:Callback URL token" bson:"callback_token,omitempty"`
	MerkleProof   bool              `json:"merkle_proof" toml:"merkle_proof" yaml:"merkle_proof" gorm:"<-:create;comment:Whether to callback with a merkle proof" bson:"merkle_proof,omitempty"`

	// Private for internal use
	parsedTx *bt.Tx `gorm:"-" bson:"-"` // The go-bt version of the transaction
}

// NewTransaction will start a new transaction model
func NewTransaction(opts ...ModelOps) *Transaction {
	tx := &Transaction{
		Model: *NewBaseModel(ModelNamePolicy, opts...),
	}

	if tx.IsNew() {
		tx.Status = TransactionStatusNew
	}

	return tx
}

func NewTransactionFromHex(hexString string, opts ...ModelOps) (tx *Transaction, err error) {
	tx = NewTransaction(opts...)
	if tx.parsedTx, err = bt.NewTxFromString(hexString); err != nil {
		return
	}

	tx.ID = tx.parsedTx.TxID()
	tx.Tx = tx.parsedTx.Bytes()

	return
}

func NewTransactionFromBytes(txBytes []byte, opts ...ModelOps) (tx *Transaction, err error) {
	tx = NewTransaction(opts...)
	if tx.parsedTx, err = bt.NewTxFromBytes(txBytes); err != nil {
		return
	}

	tx.ID = tx.parsedTx.TxID()
	tx.Tx = tx.parsedTx.Bytes()

	return
}

func GetTransaction(ctx context.Context, id string, opts ...ModelOps) (*Transaction, error) {
	transaction := &Transaction{
		Model: *NewBaseModel(ModelNamePolicy, opts...),
	}
	conditions := map[string]interface{}{
		"id": id,
	}
	if err := transaction.Client().Datastore().GetModel(ctx, transaction, conditions, 5*time.Second, false); err != nil {
		return nil, err
	}

	return transaction, nil
}

func (t *Transaction) Validate() (int, *mapi.Error) {

	// no error is thrown on validation
	return 0, nil
}

func (t *Transaction) SubmitToNodes() (uint, []string, error) {
	// added to mempool
	return 201, nil, nil
}

func (t *Transaction) GetModelName() string {
	return ModelNameTransaction.String()
}

func (t *Transaction) GetModelTableName() string {
	return TableNameTransaction.String()
}

func (t *Transaction) Migrate(client datastore.ClientInterface) error {
	return client.IndexMetadata(TableNamePolicy.String(), mapi.MetadataField)
}

func (t *Transaction) Save(ctx context.Context) (err error) {
	return Save(ctx, t)
}
