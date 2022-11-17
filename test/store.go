package test

import (
	"context"
	"encoding/json"
	"time"

	"github.com/mrz1836/go-datastore"
	"go.mongodb.org/mongo-driver/mongo"
	"gorm.io/gorm"
)

type Datastore struct {
	CreateInBatchesError error
	ExecuteResult        []*gorm.DB
	GetModelResult       []interface{}
	SaveModelInput       []interface{}
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

func (d *Datastore) GetModel(_ context.Context, model interface{}, _ map[string]interface{},
	_ time.Duration, _ bool) error {

	if d.GetModelResult != nil && len(d.GetModelResult) > 0 {
		// pop the first result of the stack and return it
		var m interface{}
		m, d.GetModelResult = d.GetModelResult[0], d.GetModelResult[1:]

		if m != nil {
			// model is a pointer to a model struct
			mm, _ := json.Marshal(m)
			_ = json.Unmarshal(mm, model)
			return nil
		}
	}

	return datastore.ErrNoResults
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
	tx := &datastore.Transaction{}
	return fn(tx)
}

func (d *Datastore) Raw(query string) *gorm.DB {
	return nil
}

func (d *Datastore) SaveModel(_ context.Context, model interface{}, _ *datastore.Transaction, _, _ bool) error {

	d.SaveModelInput = append(d.SaveModelInput, model)

	return nil
}

func (d *Datastore) GetArrayFields() []string {
	//TODO implement me
	panic("implement me")
}

func (d *Datastore) GetDatabaseName() string {
	//TODO implement me
	panic("implement me")
}

func (d *Datastore) GetMongoCollection(collectionName string) *mongo.Collection {
	//TODO implement me
	panic("implement me")
}

func (d *Datastore) GetMongoCollectionByTableName(tableName string) *mongo.Collection {
	//TODO implement me
	panic("implement me")
}

func (d *Datastore) GetMongoConditionProcessor() func(conditions *map[string]interface{}) {
	//TODO implement me
	panic("implement me")
}

func (d *Datastore) GetMongoIndexer() func() map[string][]mongo.IndexModel {
	//TODO implement me
	panic("implement me")
}

func (d *Datastore) GetObjectFields() []string {
	//TODO implement me
	panic("implement me")
}

func (d *Datastore) GetTableName(modelName string) string {
	//TODO implement me
	panic("implement me")
}

func (d *Datastore) Close(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (d *Datastore) Debug(on bool) {
	//TODO implement me
	panic("implement me")
}

func (d *Datastore) DebugLog(ctx context.Context, text string) {
	//TODO implement me
	panic("implement me")
}

func (d *Datastore) Engine() datastore.Engine {
	return "mock"
}

func (d *Datastore) IsAutoMigrate() bool {
	//TODO implement me
	panic("implement me")
}

func (d *Datastore) IsDebug() bool {
	//TODO implement me
	panic("implement me")
}

func (d *Datastore) IsNewRelicEnabled() bool {
	//TODO implement me
	panic("implement me")
}
