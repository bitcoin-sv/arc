package models

import (
	"time"

	"github.com/TAAL-GmbH/mapi/client"
)

// enrich is run after getting a record from the database
func (m *Model) enrich(name ModelName, opts ...ModelOps) {
	// Set the name
	m.name = name

	// Overwrite defaults
	m.SetOptions(opts...)
}

func (m *Model) Client() client.Interface {
	return m.client
}

func (m *Model) DebugLog(text string) {
	//TODO implement me
	panic("implement me")
}

// GetOptions will get the options that are set on that model
func (m *Model) GetOptions(isNewRecord bool) (opts []ModelOps) {

	// Client was set on the model
	/*
		if m.client != nil {
			opts = append(opts, WithClient(m.client))
		}
	*/

	// New record flag
	if isNewRecord {
		opts = append(opts, New())
	}

	return
}

// IsNew returns true if the model is (or was) a new record
func (m *Model) IsNew() bool {
	return m.newRecord
}

// GetID will get the model id, if overwritten in the actual model
func (m *Model) GetID() string {
	return ""
}

// Name will get the collection name (model)
func (m *Model) Name() string {
	return m.name.String()
}

// New will set the record to new
func (m *Model) New() {
	m.newRecord = true
}

// NotNew sets newRecord to false
func (m *Model) NotNew() {
	m.newRecord = false
}

// SetRecordTime will set the record timestamps (created is true for a new record)
func (m *Model) SetRecordTime(created bool) {
	if created {
		m.CreatedAt = time.Now().UTC()
	} else {
		m.UpdatedAt = time.Now().UTC()
	}
}

// UpdateMetadata will update the metadata on the model
// any key set to nil will be removed, other keys updated or added
func (m *Model) UpdateMetadata(metadata Metadata) {
	if m.Metadata == nil {
		m.Metadata = make(Metadata)
	}

	for key, value := range metadata {
		if value == nil {
			delete(m.Metadata, key)
		} else {
			m.Metadata[key] = value
		}
	}
}

// SetOptions will set the options on the model
func (m *Model) SetOptions(opts ...ModelOps) {
	for _, opt := range opts {
		opt(m)
	}
}
