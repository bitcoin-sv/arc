package dynamodb

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/metamorph/store"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/log"
	"github.com/ordishs/gocore"
	"github.com/spf13/viper"
)

type DynamoDB struct {
	dynamoCli *dynamodb.Client
}

const ISO8601 = "2006-01-02T15:04:05.999Z"

func New() (store.MetamorphStore, error) {
	// export necessary parameters for aws dynamodb connection
	err := os.Setenv("AWS_ACCESS_KEY_ID", viper.GetString("metamorph.db.dynamodb.aws_access_key_id"))
	if err != nil {
		return &DynamoDB{}, err
	}
	err = os.Setenv("AWS_SECRET_ACCESS_KEY", viper.GetString("metamorph.db.dynamodb.aws_secret_access_key"))
	if err != nil {
		return &DynamoDB{}, err
	}
	err = os.Setenv("AWS_SESSION_TOKEN", viper.GetString("metamorph.db.dynamodb.aws_session_token"))
	if err != nil {
		return &DynamoDB{}, err
	}

	// Using the SDK's default configuration, loading additional config
	// and credentials values from the environment variables, shared
	// credentials, and shared configuration files
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(os.Getenv("AWS_REGION")))
	if err != nil {
		return &DynamoDB{}, err
	}

	// create dynamodb client
	dynamodbClient := &DynamoDB{dynamoCli: dynamodb.NewFromConfig(cfg)}

	// create table if not exists
	exists, err := dynamodbClient.TableExists("transactions")
	if err != nil {
		return dynamodbClient, err
	}

	if !exists {
		err = dynamodbClient.CreateTransactionsTable()
		if err != nil {
			return dynamodbClient, err
		}
	}

	// create table if not exists
	exists, err = dynamodbClient.TableExists("blocks")
	if err != nil {
		return dynamodbClient, err
	}

	if !exists {
		err = dynamodbClient.CreateBlocksTable()
		if err != nil {
			return dynamodbClient, err
		}
	}

	// Using the Config value, create the DynamoDB client
	return dynamodbClient, nil
}

func (ddb *DynamoDB) TableExists(tableName string) (bool, error) {
	// Build the request with its input parameters
	resp, err := ddb.dynamoCli.ListTables(context.TODO(), &dynamodb.ListTablesInput{})
	if err != nil {
		return false, err
	}

	for _, tn := range resp.TableNames {
		if tn == tableName {

			return true, nil
		}
	}
	return false, nil
}

// CreateTransactionsTable creates a DynamoDB table for storing metamorph user transactions
func (ddb *DynamoDB) CreateTransactionsTable() error {
	// construct primary key and create table
	_, err := ddb.dynamoCli.CreateTable(context.TODO(), &dynamodb.CreateTableInput{
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("tx_hash"),
				AttributeType: types.ScalarAttributeTypeB,
			},
			{
				AttributeName: aws.String("tx_status_shard"),
				AttributeType: types.ScalarAttributeTypeN,
			},
			{
				AttributeName: aws.String("tx_status"),
				AttributeType: types.ScalarAttributeTypeN,
			},
		},
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("tx_hash"),
				KeyType:       types.KeyTypeHash,
			},
		},
		GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
			{
				IndexName: aws.String("statuses"),
				KeySchema: []types.KeySchemaElement{
					{
						AttributeName: aws.String("tx_status_shard"),
						KeyType:       types.KeyTypeHash,
					},
					{
						AttributeName: aws.String("tx_status"),
						KeyType:       types.KeyTypeRange,
					},
				},
				Projection: &types.Projection{
					ProjectionType: types.ProjectionTypeAll,
				},
				ProvisionedThroughput: &types.ProvisionedThroughput{
					ReadCapacityUnits:  aws.Int64(10),
					WriteCapacityUnits: aws.Int64(10),
				},
			},
		},
		TableName: aws.String("transactions"),
		ProvisionedThroughput: &types.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(10),
			WriteCapacityUnits: aws.Int64(10),
		},
	})

	if err != nil {
		return err
	}

	return nil
}

// CreateBlocksTable creates a DynamoDB table for storing blocks
func (ddb *DynamoDB) CreateBlocksTable() error {
	// construct primary key and create table
	if _, err := ddb.dynamoCli.CreateTable(context.TODO(), &dynamodb.CreateTableInput{
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("block_hash"),
				AttributeType: types.ScalarAttributeTypeB,
			},
		},
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("block_hash"),
				KeyType:       types.KeyTypeHash,
			}},
		TableName: aws.String("blocks"),
		ProvisionedThroughput: &types.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(10),
			WriteCapacityUnits: aws.Int64(10),
		},
	}); err != nil {
		return err
	}

	return nil
}

func (ddb *DynamoDB) Get(ctx context.Context, key []byte) (*store.StoreData, error) {
	// config log and tracing
	startNanos := time.Now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_dynamodb").NewStat("Get").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "dynamodb:Get")
	defer span.Finish()

	val := map[string]types.AttributeValue{
		"tx_hash": &types.AttributeValueMemberB{Value: key},
	}

	response, err := ddb.dynamoCli.GetItem(ctx, &dynamodb.GetItemInput{
		Key: val, TableName: aws.String("transactions"),
	})
	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return nil, err
	}

	if len(response.Item) != 0 {
		var transaction store.StoreData
		err = attributevalue.UnmarshalMap(response.Item, &transaction)
		if err != nil {
			span.SetTag(string(ext.Error), true)
			span.LogFields(log.Error(err))
			return nil, err
		}

		return &transaction, nil
	}

	return nil, store.ErrNotFound
}

func (ddb *DynamoDB) Set(ctx context.Context, key []byte, value *store.StoreData) error {
	// setup log and tracing
	startNanos := time.Now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_dynamodb").NewStat("Set").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "dynamodb:Set")
	defer span.Finish()

	// marshal input value for new entry
	item, err := attributevalue.MarshalMap(value)
	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return err
	}

	// put item into table
	_, err = ddb.dynamoCli.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String("transactions"), Item: item,
	})

	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return err
	}

	return nil
}

func (ddb *DynamoDB) Del(ctx context.Context, key []byte) error {
	// setup log and tracing
	startNanos := time.Now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_dynamodb").NewStat("Del").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "dynamodb:Del")
	defer span.Finish()

	// delete the item
	val, err := attributevalue.MarshalMap(key)
	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return err
	}

	_, err = ddb.dynamoCli.DeleteItem(context.TODO(), &dynamodb.DeleteItemInput{
		TableName: aws.String("transactions"), Key: val,
	})

	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return err
	}

	return nil

}

func (ddb *DynamoDB) GetUnmined(ctx context.Context, callback func(s *store.StoreData)) error {
	// setup log and tracing
	startNanos := time.Now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_dynamodb").NewStat("getunmined").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "dynamodb:GetUnmined")
	defer span.Finish()

	// get unmined set
	out, err := ddb.dynamoCli.Query(context.TODO(), &dynamodb.QueryInput{
		TableName:              aws.String("transactions"),
		IndexName:              aws.String("statuses"),
		KeyConditionExpression: aws.String("tx_status_shard = :tx_status_shard and tx_status < :tx_status"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":tx_status":       &types.AttributeValueMemberN{Value: strconv.Itoa(int(metamorph_api.Status_MINED))},
			":tx_status_shard": &types.AttributeValueMemberN{Value: strconv.Itoa(int(1))},
		},
	})

	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return err
	}

	for _, item := range out.Items {
		var transaction store.StoreData
		err = attributevalue.UnmarshalMap(item, &transaction)
		if err != nil {
			span.SetTag(string(ext.Error), true)
			span.LogFields(log.Error(err))
			return err
		}

		callback(&transaction)
	}
	return nil
}

func (ddb *DynamoDB) UpdateStatus(ctx context.Context, hash *chainhash.Hash, status metamorph_api.Status, rejectReason string) error {
	// setup log and tracing
	startNanos := time.Now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("UpdateStatus").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:UpdateStatus")
	defer span.Finish()

	// update tx
	_, err := ddb.dynamoCli.UpdateItem(context.TODO(), &dynamodb.UpdateItemInput{
		TableName: aws.String("transactions"),
		Key: map[string]types.AttributeValue{
			"tx_hash": &types.AttributeValueMemberB{Value: hash.CloneBytes()},
		},
		UpdateExpression: aws.String("set tx_status = :tx_status, reject_reason = :reject_reason"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":tx_status":     &types.AttributeValueMemberN{Value: strconv.Itoa(int(status))},
			":reject_reason": &types.AttributeValueMemberS{Value: rejectReason},
		},
	})

	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return err
	}

	return nil
}

func (ddb *DynamoDB) UpdateMined(ctx context.Context, hash *chainhash.Hash, blockHash *chainhash.Hash, blockHeight uint64) error {
	// setup log and tracing
	startNanos := time.Now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_dynamodb").NewStat("UpdateMined").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "dynamodb:UpdateMined")
	defer span.Finish()

	// set block parameters and status - mined
	_, err := ddb.dynamoCli.UpdateItem(context.TODO(), &dynamodb.UpdateItemInput{
		TableName: aws.String("transactions"),
		Key: map[string]types.AttributeValue{
			"tx_hash": &types.AttributeValueMemberB{Value: hash.CloneBytes()},
		},
		UpdateExpression: aws.String("set block_height = :block_height, block_hash = :block_hash, tx_status = :tx_status"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":block_hash":   &types.AttributeValueMemberB{Value: blockHash.CloneBytes()},
			":block_height": &types.AttributeValueMemberN{Value: strconv.Itoa(int(blockHeight))},
			":tx_status":    &types.AttributeValueMemberN{Value: strconv.Itoa(int(metamorph_api.Status_MINED))},
		},
	})

	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return err
	}

	return nil
}

// marshal input value for new entry
type BlockItem struct {
	Hash        []byte `dynamodbav:"block_hash"`
	ProcessedAt string `dynamodbav:"processed_at"`
}

func (ddb *DynamoDB) GetBlockProcessed(ctx context.Context, blockHash *chainhash.Hash) (*time.Time, error) {
	// setup log and tracing
	startNanos := time.Now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_dynamodb").NewStat("GetBlockProcessed").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "dynamodb:GetBlockProcessed")
	defer span.Finish()

	val := map[string]types.AttributeValue{
		"block_hash": &types.AttributeValueMemberB{Value: blockHash.CloneBytes()},
	}

	response, err := ddb.dynamoCli.GetItem(ctx, &dynamodb.GetItemInput{
		Key: val, TableName: aws.String("blocks"),
	})

	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return nil, err
	}

	var blockItem BlockItem
	err = attributevalue.UnmarshalMap(response.Item, &blockItem)
	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return nil, err
	}

	var processedAtTime time.Time
	if blockItem.ProcessedAt != "" {
		processedAtTime, err = time.Parse(ISO8601, blockItem.ProcessedAt)
		if err != nil {
			span.SetTag(string(ext.Error), true)
			span.LogFields(log.Error(err))
			return nil, err
		}
	}

	return &processedAtTime, nil
}

func (ddb *DynamoDB) SetBlockProcessed(ctx context.Context, blockHash *chainhash.Hash) error {
	// set log and tracing
	startNanos := time.Now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_dynamodb").NewStat("SetBlockProcessed").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "dynamodb:SetBlockProcessed")
	defer span.Finish()

	blockItem := BlockItem{Hash: blockHash.CloneBytes(), ProcessedAt: time.Now().UTC().Format(ISO8601)}
	item, err := attributevalue.MarshalMap(blockItem)
	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return err
	}

	// put item into table
	_, err = ddb.dynamoCli.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String("blocks"), Item: item,
	})

	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return err
	}

	return nil
}

func (ddb *DynamoDB) Close(ctx context.Context) error {
	startNanos := time.Now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_dynamodb").NewStat("Close").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "dynamodb:Close")
	defer span.Finish()

	ctx.Done()
	return nil
}
