package models

import (
	"context"
	"time"

	"github.com/labstack/gommon/random"
	"github.com/mrz1836/go-datastore"
	"github.com/taal/mapi"
)

// ModelNamePolicy defines the model name
var ModelNamePolicy ModelName = "policy"
var TableNamePolicy ModelName = "policies"

// Policy defines the database model for fees
type Policy struct {
	Model `bson:",inline"`

	ID       string   `json:"id" toml:"id" yaml:"id" gorm:"<-:create;type:char(64);primaryKey;comment:This is the unique hash of the fee" bson:"_id"`
	Fees     Fees     `json:"fees" toml:"fees" yaml:"fees" gorm:"<-:create;type:text;comment:Fees object for" bson:"fees"`
	Policies Policies `json:"policies" toml:"policies" yaml:"policies" gorm:"<-:create;type:text;comment:Policies" bson:"policies"`
	ClientID string   `json:"client_id,omitempty" toml:"client_id" yaml:"client_id" gorm:"<-:create;unique;comment:Optional clientID for this record" bson:"client_id"`
}

// NewPolicy will start a new policy model
func NewPolicy(opts ...ModelOps) *Policy {
	return &Policy{
		ID:    random.String(32, random.Hex),
		Model: *NewBaseModel(ModelNamePolicy, opts...),
	}
}

func GetPolicy(ctx context.Context, id string, opts ...ModelOps) (*Policy, error) {
	fee := &Policy{
		Model: *NewBaseModel(ModelNamePolicy, opts...),
	}
	conditions := map[string]interface{}{
		"id": id,
	}
	if err := fee.Client().Datastore().GetModel(ctx, fee, conditions, 5*time.Second, false); err != nil {
		return nil, err
	}

	return fee, nil
}

func GetDefaultPolicy(ctx context.Context, opts ...ModelOps) (*Policy, error) {
	return GetPolicyForClient(ctx, "", opts...)
}

func GetPolicyForClient(ctx context.Context, clientID string, opts ...ModelOps) (*Policy, error) {
	model := &Policy{Model: *NewBaseModel(ModelNamePolicy, opts...)}
	conditions := map[string]interface{}{
		"client_id": clientID,
	}

	var fee *Policy
	if err := model.Client().Datastore().GetModel(ctx, &fee, conditions, 5*time.Second, false); err != nil {
		return nil, err
	}

	return fee, nil
}

func (p *Policy) GetModelName() string {
	return ModelNamePolicy.String()
}

func (m *Model) GetModelTableName() string {
	return TableNamePolicy.String()
}

func (m *Model) Migrate(client datastore.ClientInterface) error {
	return client.IndexMetadata(TableNamePolicy.String(), mapi.MetadataField)
}

func (p *Policy) Save(ctx context.Context) (err error) {
	return Save(ctx, p)
}
