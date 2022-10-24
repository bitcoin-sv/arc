package models

import (
	"context"

	"github.com/labstack/gommon/random"
	"github.com/mrz1836/go-datastore"
	"github.com/taal/mapi"
)

// ModelNameLogAccess defines the model name
var ModelNameLogAccess ModelName = "access_log"
var TableNameLogAccess ModelName = "access_logs"

// LogAccess defines the database model for the access log
type LogAccess struct {
	Model `bson:",inline"`

	ID           string `json:"id" toml:"id" yaml:"id" gorm:"<-:create;type:char(32);primaryKey;comment:This is the unique id of the record" bson:"_id"`
	IP           string `json:"ip" toml:"ip" yaml:"ip" gorm:"<-:create;type:varchar(32);comment:The ip of the requesting host" bson:"ip"`
	Host         string `json:"host" toml:"host" yaml:"host" gorm:"<-:create;type:text;comment:The host that was accesssed" bson:"host"`
	Method       string `json:"method" toml:"method" yaml:"method" gorm:"<-:create;type:varchar(6);comment:HTTP method called" bson:"method"`
	RequestURI   string `json:"request_uri" toml:"request_uri" yaml:"request_uri" gorm:"<-:create;type:varchar(255);comment:The uri that was requested" bson:"request_uri"`
	Referer      string `json:"referer" toml:"referer" yaml:"referer" gorm:"<-:create;type:text;comment:The data related to the action" bson:"referer"`
	UserAgent    string `json:"user_agent" toml:"user_agent" yaml:"user_agent" gorm:"<-:create;type:text;comment:The data related to the action" bson:"user_agent"`
	ClientID     string `json:"client_id" toml:"client_id" yaml:"client_id" gorm:"<-:create;type:char(32);comment:The id of the client doing the action" bson:"client_id"`
	RequestData  string `json:"request_data" toml:"request_data" yaml:"request_data" gorm:"<-:create;type:text;comment:The data in the request" bson:"request_data"`
	ResponseData string `json:"response_data" toml:"response_data" yaml:"response_data" gorm:"<-:create;type:text;comment:The data sent back to the requesting party" bson:"response_data"`
}

// NewLogAccess will start a new access log model
func NewLogAccess(opts ...ModelOps) *LogAccess {
	return &LogAccess{
		ID:    random.String(32, random.Hex),
		Model: *NewBaseModel(ModelNameLogAccess, opts...),
	}
}

func (l *LogAccess) GetModelName() string {
	return ModelNameLogAccess.String()
}

func (l *LogAccess) GetModelTableName() string {
	return TableNameLogAccess.String()
}

func (l *LogAccess) Migrate(client datastore.ClientInterface) error {
	return client.IndexMetadata(TableNamePolicy.String(), mapi.MetadataField)
}

func (l *LogAccess) Save(ctx context.Context) (err error) {
	return Save(ctx, l)
}
