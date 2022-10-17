package models

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/mrz1836/go-datastore"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type FeeAmount struct {
	Bytes    uint64 `json:"bytes,omitempty"`
	Satoshis uint64 `json:"satoshis,omitempty"`
}

// GormDataType type in gorm
func (f FeeAmount) GormDataType() string {
	return "text"
}

// Scan scan value into Json, implements sql.Scanner interface
func (f *FeeAmount) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	xType := fmt.Sprintf("%T", value)
	var byteValue []byte
	if xType == "string" {
		byteValue = []byte(value.(string))
	} else {
		byteValue = value.([]byte)
	}
	if bytes.Equal(byteValue, []byte("")) || bytes.Equal(byteValue, []byte("\"\"")) {
		return nil
	}

	return json.Unmarshal(byteValue, &f)
}

// Value return json value, implement driver.Valuer interface
func (f FeeAmount) Value() (driver.Value, error) {
	marshal, err := json.Marshal(f)
	if err != nil {
		return nil, err
	}

	return string(marshal), nil
}

// GormDBDataType the gorm data type for metadata
func (FeeAmount) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	if db.Dialector.Name() == datastore.Postgres {
		return datastore.JSONB
	}
	return datastore.JSON
}

// MarshalBSONValue method is called by bson.Marshal in Mongo for type = Metadata
func (f *FeeAmount) MarshalBSONValue() (bsontype.Type, []byte, error) {
	if f == nil {
		return bson.TypeNull, nil, nil
	}

	return bson.MarshalValue(f)
}

// UnmarshalBSONValue method is called by bson.Unmarshal in Mongo for type = Metadata
func (f *FeeAmount) UnmarshalBSONValue(t bsontype.Type, data []byte) error {
	raw := bson.RawValue{Type: t, Value: data}

	if raw.Value == nil {
		return nil
	}

	if err := raw.Unmarshal(&f); err != nil {
		return err
	}

	return nil
}
