package models

import (
	"context"

	"github.com/TAAL-GmbH/mapi"
	"github.com/labstack/gommon/random"
	"github.com/mrz1836/go-datastore"
)

// ModelNameLogError defines the model name
var ModelNameLogError ModelName = "error_log"
var TableNameLogError ModelName = "error_logs"

// LogError defines the database model for the access log
type LogError struct {
	Model `bson:",inline"`

	ID          string            `json:"id" toml:"id" yaml:"id" gorm:"<-:create;type:char(32);primaryKey;comment:This is the unique id of the record" bson:"_id"`
	LogAccessID string            `json:"log_access_id" toml:"log_access_id" yaml:"log_access_id" gorm:"<-:create;type:char(32);comment:The log access id" bson:"log_access_id"`
	Error       *mapi.ErrorFields `json:"error" toml:"error" yaml:"error" gorm:"<-:create;type:text;comment:The error body returned in the http request" bson:"error"`
	TxID        string            `json:"tx_id" toml:"tx_id" yaml:"tx_id" gorm:"<-:create;type:char(32);comment:The transaction id of the transaction causing the error" bson:"tx_id"`
	ClientID    string            `json:"client_id" toml:"client_id" yaml:"client_id" gorm:"<-:create;type:char(32);comment:The id of the client" bson:"client_id"`
}

// NewLogError will start a new access log model
func NewLogError(opts ...ModelOps) *LogError {
	return &LogError{
		ID:    random.String(32, random.Hex),
		Model: *NewBaseModel(ModelNameLogAccess, opts...),
	}
}

func (l *LogError) GetModelName() string {
	return ModelNameLogError.String()
}

func (l *LogError) GetModelTableName() string {
	return TableNameLogError.String()
}

func (l *LogError) Migrate(client datastore.ClientInterface) error {
	return client.IndexMetadata(TableNamePolicy.String(), mapi.MetadataField)
}

func (l *LogError) Save(ctx context.Context) (err error) {
	return Save(ctx, l)
}
