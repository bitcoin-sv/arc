package models

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/TAAL-GmbH/mapi"
	"github.com/mrz1836/go-datastore"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// Fees struct to tell database systems how to marshal and unmarshal
type Fees []mapi.Fee

// GormDataType type in gorm
func (f Fees) GormDataType() string {
	return "text"
}

// Scan scan value into Json, implements sql.Scanner interface
func (f *Fees) Scan(value interface{}) error {
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
func (f Fees) Value() (driver.Value, error) {
	marshal, err := json.Marshal(f)
	if err != nil {
		return nil, err
	}

	return string(marshal), nil
}

// GormDBDataType the gorm data type for fees
func (Fees) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	if db.Dialector.Name() == datastore.Postgres {
		return datastore.JSONB
	}
	return datastore.JSON
}

// MarshalBSONValue method is called by bson.Marshal in Mongo for type = Fees
func (f *Fees) MarshalBSONValue() (bsontype.Type, []byte, error) {
	if f == nil {
		return bson.TypeNull, nil, nil
	}

	// just let Mongo unmarshal into map
	fees := make([]map[string]interface{}, 0)
	b, err := json.Marshal(f)
	if err != nil {
		return bson.TypeNull, nil, err
	}
	if err = json.Unmarshal(b, &fees); err != nil {
		return bson.TypeNull, nil, err
	}

	return bson.MarshalValue(fees)
}

// UnmarshalBSONValue method is called by bson.Unmarshal in Mongo for type = Fees
func (f *Fees) UnmarshalBSONValue(t bsontype.Type, data []byte) error {
	raw := bson.RawValue{Type: t, Value: data}

	if raw.Value == nil {
		return nil
	}

	var fees []map[string]interface{}
	if err := raw.Unmarshal(&fees); err != nil {
		return err
	}

	*f = make(Fees, 0)
	for _, fee := range fees {
		b, err := json.Marshal(fee)
		if err != nil {
			return err
		}
		var mf mapi.Fee
		if err = json.Unmarshal(b, &mf); err != nil {
			return err
		}
		*f = append(*f, mf)
	}

	return nil
}
