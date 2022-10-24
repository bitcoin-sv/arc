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

type Policies map[string]interface{}

// GormDataType type in gorm
func (p Policies) GormDataType() string {
	return "text"
}

// Scan scan value into Json, implements sql.Scanner interface
func (p *Policies) Scan(value interface{}) error {
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

	return json.Unmarshal(byteValue, &p)
}

// Value return json value, implement driver.Valuer interface
func (p Policies) Value() (driver.Value, error) {
	marshal, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}

	return string(marshal), nil
}

// GormDBDataType the gorm data type for fees
func (Policies) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	if db.Dialector.Name() == datastore.Postgres {
		return datastore.JSONB
	}
	return datastore.JSON
}

// MarshalBSONValue method is called by bson.Marshal in Mongo for type = Fees
func (p *Policies) MarshalBSONValue() (bsontype.Type, []byte, error) {
	if p == nil {
		return bson.TypeNull, nil, nil
	}

	// just let Mongo marshal into map
	policies := make(map[string]interface{})
	b, err := json.Marshal(p)
	if err != nil {
		return bson.TypeNull, nil, err
	}
	if err = json.Unmarshal(b, &policies); err != nil {
		return bson.TypeNull, nil, err
	}

	return bson.MarshalValue(policies)
}

// UnmarshalBSONValue method is called by bson.Unmarshal in Mongo for type = Fees
func (p *Policies) UnmarshalBSONValue(t bsontype.Type, data []byte) error {
	raw := bson.RawValue{Type: t, Value: data}

	if raw.Value == nil {
		return nil
	}

	var policies map[string]interface{}
	if err := raw.Unmarshal(&policies); err != nil {
		return err
	}

	*p = make(Policies)
	for key, policy := range policies {
		(*p)[key] = policy
	}

	return nil
}
