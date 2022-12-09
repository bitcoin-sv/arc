package models

import (
	"context"
	"time"

	"github.com/TAAL-GmbH/arc/client"
	"github.com/mrz1836/go-datastore"
	customTypes "github.com/mrz1836/go-datastore/custom_types"
)

var defaultPageSize = 25

// Model is the generic model field(s) and interface(s)
//
// gorm: https://gorm.io/docs/models.html
type Model struct {
	CreatedAt time.Time            `json:"created_at" toml:"created_at" yaml:"created_at" gorm:"comment:The time that the record was originally created" bson:"created_at"`
	UpdatedAt time.Time            `json:"updated_at" toml:"updated_at" yaml:"updated_at" gorm:"comment:The time that the record was last updated" bson:"updated_at,omitempty"`
	DeletedAt customTypes.NullTime `json:"deleted_at" toml:"deleted_at" yaml:"deleted_at" gorm:"index;comment:The time the record was marked as deleted" bson:"deleted_at,omitempty"`
	Metadata  Metadata             `gorm:"type:json;comment:The JSON metadata for the record" json:"metadata,omitempty" bson:"metadata,omitempty"`

	// Internal fields
	client    client.Interface // Interface of the client being used
	name      ModelName        // Name of model (table name)
	newRecord bool             // Determine if the record is new (create vs update)
	pageSize  int              // Number of items per page to get if being used in for method getModels
}

// ModelInterface is the interface that all models share
type ModelInterface interface {
	Client() client.Interface
	DebugLog(text string)
	GetID() string
	GetModelName() string
	GetModelTableName() string
	IsNew() bool
	Migrate(client datastore.ClientInterface) error
	Name() string
	New()
	NotNew()
	Save(ctx context.Context) (err error)
	SetOptions(opts ...ModelOps)
	SetRecordTime(created bool)
	UpdateMetadata(metadata Metadata)
}

// NewBaseModel create an empty base model
func NewBaseModel(name ModelName, opts ...ModelOps) (m *Model) {
	m = &Model{
		name:     name,
		pageSize: defaultPageSize,
	}
	m.SetOptions(opts...)

	return
}

// ModelName is the model name type
type ModelName string

// String is the string version of the name
func (n ModelName) String() string {
	return string(n)
}

// IsEmpty tests if the model name is empty
func (n ModelName) IsEmpty() bool {
	return n == "empty"
}

// TableName is the model db table name type
type TableName string

// String is the string version of the name
func (t TableName) String() string {
	return string(t)
}
