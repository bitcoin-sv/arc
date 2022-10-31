package test

import (
	"context"
	"time"

	"github.com/mrz1836/go-datastore"
	"gorm.io/gorm"
)

type Datastore struct {
	CreateInBatchesError error
	ExecuteResult        []*gorm.DB
}

func (d *Datastore) AutoMigrateDatabase(_ context.Context, _ ...interface{}) error {
	return nil
}

func (d *Datastore) CreateInBatches(_ context.Context, models interface{}, _ int) error {
	return d.CreateInBatchesError
}

func (d *Datastore) CustomWhere(_ datastore.CustomWhereInterface, _ map[string]interface{}, _ datastore.Engine) interface{} {
	return nil
}

func (d *Datastore) Execute(query string) *gorm.DB {
	if d.ExecuteResult != nil {
		var g *gorm.DB
		// pop the first result of the stack and return it
		g, d.ExecuteResult = d.ExecuteResult[0], d.ExecuteResult[1:]
		return g
	}

	return nil
}

func (d *Datastore) GetModel(ctx context.Context, model interface{}, conditions map[string]interface{},
	timeout time.Duration, forceWriteDB bool) error {

	return nil
}

func (d *Datastore) GetModels(ctx context.Context, models interface{}, conditions map[string]interface{}, queryParams *datastore.QueryParams,
	fieldResults interface{}, timeout time.Duration) error {
	return nil
}

func (d *Datastore) GetModelCount(ctx context.Context, model interface{}, conditions map[string]interface{},
	timeout time.Duration) (int64, error) {
	return 0, nil
}

func (d *Datastore) GetModelsAggregate(ctx context.Context, models interface{}, conditions map[string]interface{},
	aggregateColumn string, timeout time.Duration) (map[string]interface{}, error) {

	return nil, nil
}

func (d *Datastore) HasMigratedModel(modelType string) bool {
	return false
}

func (d *Datastore) IncrementModel(ctx context.Context, model interface{},
	fieldName string, increment int64) (newValue int64, err error) {

	return 0, nil
}

func (d *Datastore) IndexExists(tableName, indexName string) (bool, error) {
	return false, nil
}

func (d *Datastore) IndexMetadata(tableName, field string) error {
	return nil
}

func (d *Datastore) NewTx(ctx context.Context, fn func(*datastore.Transaction) error) error {
	return nil
}

func (d *Datastore) Raw(query string) *gorm.DB {
	return nil
}

func (d *Datastore) SaveModel(ctx context.Context, model interface{}, tx *datastore.Transaction, newRecord, commitTx bool) error {
	return nil
}
