package models

import (
	"context"
	"time"

	"github.com/labstack/gommon/random"
	"github.com/mrz1836/go-datastore"
)

// ModelNameFee defines the model name
var ModelNameFee ModelName = "fee"

// Fee defines the database model for fees
type Fee struct {
	Model `bson:",inline"`

	ID        string    `json:"id" toml:"id" yaml:"id" gorm:"<-:create;type:char(64);primaryKey;comment:This is the unique hash of the fee" bson:"_id"`
	FeeType   string    `json:"fee_type" toml:"fee_type" yaml:"fee_type" gorm:"<-;comment:The fee type of this record" bson:"fee_type"`
	MiningFee FeeAmount `json:"mining_fee" toml:"mining_fee" yaml:"mining_fee" gorm:"<-type:json;comment:The mining fee for this type of tx" bson:"mining_fee"`
	RelayFee  FeeAmount `json:"relay_fee" toml:"relay_fee" yaml:"relay_fee" gorm:"<-type:json;comment:The relay fee for this type of tx" bson:"relay_fee"`
	ClientID  string    `json:"client_id,omitempty" toml:"client_id" yaml:"client_id" gorm:"<-;comment:Optional clientID for this record" bson:"client_id,omitempty"`
}

// NewFee will start a new fee model
func NewFee(opts ...ModelOps) *Fee {
	return &Fee{
		ID:    random.String(32, random.Hex),
		Model: *NewBaseModel(ModelNameFee, opts...),
	}
}

func GetFee(ctx context.Context, id string, opts ...ModelOps) (*Fee, error) {
	fee := &Fee{
		Model: *NewBaseModel(ModelNameFee, opts...),
	}
	conditions := map[string]interface{}{
		"id": id,
	}
	// TODO abstract this away
	if err := fee.Client().Datastore().GetModel(ctx, fee, conditions, 5*time.Second, false); err != nil {
		return nil, err
	}

	return fee, nil
}

func GetDefaultFees(ctx context.Context, opts ...ModelOps) ([]*Fee, error) {
	model := &Fee{Model: *NewBaseModel(ModelNameFee, opts...)}
	conditions := map[string]interface{}{
		"client_id": nil,
	}

	var fees []*Fee
	// TODO abstract this away
	if err := model.Client().Datastore().GetModels(ctx, &fees, conditions, nil, nil, 5*time.Second); err != nil {
		return nil, err
	}

	return fees, nil
}

func GetFeesForClient(ctx context.Context, clientID string, opts ...ModelOps) ([]*Fee, error) {
	model := &Fee{Model: *NewBaseModel(ModelNameFee, opts...)}
	conditions := map[string]interface{}{
		"client_id": clientID,
	}

	var fees []*Fee
	// TODO abstract this away
	if err := model.Client().Datastore().GetModels(ctx, &fees, conditions, nil, nil, 5*time.Second); err != nil {
		return nil, err
	}

	return fees, nil
}

func (f *Fee) Save(ctx context.Context) (err error) {
	// TODO create helper Save function and abstract most of this away
	return f.client.Datastore().NewTx(ctx, func(tx *datastore.Transaction) error {
		return f.client.Datastore().SaveModel(ctx, f, tx, true, true)
	})
}
