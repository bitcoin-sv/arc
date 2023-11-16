package dynamodb

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
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
)

const (
	lockedByNone = "NONE"

	lockedByAttributeKey     = ":locked_by"
	txStatusAttributeKey     = ":tx_status"
	blockHeightAttributeKey  = ":block_height"
	blockHashAttributeKey    = ":block_hash"
	rejectReasonAttributeKey = ":reject_reason"
)

type DynamoDB struct {
	client   *dynamodb.Client
	hostname string
}

func New(client *dynamodb.Client, hostname string) (*DynamoDB, error) {
	repo := &DynamoDB{
		client:   client,
		hostname: hostname,
	}

	err := initialize(context.Background(), repo)

	if err != nil {
		return nil, err
	}
	return repo, nil
}

func initialize(ctx context.Context, dynamodbClient *DynamoDB) error {

	// create table if not exists
	exists, err := dynamodbClient.TableExists(ctx, "transactions")
	if err != nil {
		return err
	}

	if !exists {
		err = dynamodbClient.CreateTransactionsTable(ctx)
		if err != nil {
			return err
		}
	}

	// create table if not exists
	exists, err = dynamodbClient.TableExists(ctx, "blocks")
	if err != nil {
		return err
	}

	if !exists {
		err = dynamodbClient.CreateBlocksTable(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func (ddb *DynamoDB) TableExists(ctx context.Context, tableName string) (bool, error) {
	// Build the request with its input parameters
	resp, err := ddb.client.ListTables(ctx, &dynamodb.ListTablesInput{})
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
func (ddb *DynamoDB) CreateTransactionsTable(ctx context.Context) error {
	// construct primary key and create table
	_, err := ddb.client.CreateTable(ctx, &dynamodb.CreateTableInput{
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("tx_hash"),
				AttributeType: types.ScalarAttributeTypeB,
			},
			{
				AttributeName: aws.String("locked_by"),
				AttributeType: types.ScalarAttributeTypeS,
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
				IndexName: aws.String("locked_by_index"),
				KeySchema: []types.KeySchemaElement{
					{
						AttributeName: aws.String("locked_by"),
						KeyType:       types.KeyTypeHash,
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
func (ddb *DynamoDB) CreateBlocksTable(ctx context.Context) error {
	// construct primary key and create table
	if _, err := ddb.client.CreateTable(ctx, &dynamodb.CreateTableInput{
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

func (ddb *DynamoDB) IsCentralised() bool {
	return true
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

	response, err := ddb.client.GetItem(ctx, &dynamodb.GetItemInput{
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

	value.LockedBy = ddb.hostname

	// marshal input value for new entry
	item, err := attributevalue.MarshalMap(value)
	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return err
	}

	// put item into table
	_, err = ddb.client.PutItem(ctx, &dynamodb.PutItemInput{
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

	val := map[string]types.AttributeValue{
		"tx_hash": &types.AttributeValueMemberB{Value: key},
	}

	_, err := ddb.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String("transactions"), Key: val,
	})

	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return err
	}

	return nil
}

func (ddb *DynamoDB) SetUnlocked(ctx context.Context, hashes []*chainhash.Hash) error {
	// setup log and tracing
	startNanos := time.Now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("UpdateStatus").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:UpdateStatus")
	defer span.Finish()

	// update tx
	for _, hash := range hashes {

		_, err := ddb.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName: aws.String("transactions"),
			Key: map[string]types.AttributeValue{
				"tx_hash": &types.AttributeValueMemberB{Value: hash.CloneBytes()},
			},
			UpdateExpression: aws.String(fmt.Sprintf("SET locked_by = %s", lockedByAttributeKey)),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				lockedByAttributeKey: &types.AttributeValueMemberS{Value: lockedByNone},
			},
		})
		if err != nil {
			span.SetTag(string(ext.Error), true)
			span.LogFields(log.Error(err))
			return err
		}
	}

	return nil
}

// SetUnlockedByName sets all items to unlocked which were locked by a name
func (ddb *DynamoDB) SetUnlockedByName(ctx context.Context, lockedBy string) (int, error) {
	// setup log and tracing
	startNanos := time.Now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("UpdateStatus").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:UpdateStatus")
	defer span.Finish()

	out, err := ddb.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String("transactions"),
		IndexName:              aws.String("locked_by_index"),
		KeyConditionExpression: aws.String(fmt.Sprintf("locked_by = %s", lockedByAttributeKey)),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			lockedByAttributeKey: &types.AttributeValueMemberS{Value: lockedBy},
		},
	})
	if err != nil {
		return 0, err
	}

	numberOfUpdated := 0

	for _, item := range out.Items {
		var transaction store.StoreData
		err = attributevalue.UnmarshalMap(item, &transaction)
		if err != nil {
			return 0, err
		}

		if transaction.LockedBy == lockedByNone {
			continue
		}

		_, err := ddb.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName: aws.String("transactions"),
			Key: map[string]types.AttributeValue{
				"tx_hash": &types.AttributeValueMemberB{Value: transaction.Hash.CloneBytes()},
			},
			UpdateExpression: aws.String(fmt.Sprintf("SET locked_by = %s", lockedByAttributeKey)),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				lockedByAttributeKey: &types.AttributeValueMemberS{Value: lockedByNone},
			},
		})
		if err != nil {
			span.SetTag(string(ext.Error), true)
			span.LogFields(log.Error(err))
			return numberOfUpdated, err
		}

		numberOfUpdated++
	}

	return numberOfUpdated, nil
}

func (ddb *DynamoDB) setLocked(ctx context.Context, hash *chainhash.Hash) error {
	_, err := ddb.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String("transactions"),
		Key: map[string]types.AttributeValue{
			"tx_hash": &types.AttributeValueMemberB{Value: hash.CloneBytes()},
		},
		UpdateExpression: aws.String(fmt.Sprintf("SET locked_by = %s", lockedByAttributeKey)),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			lockedByAttributeKey: &types.AttributeValueMemberS{Value: ddb.hostname},
		},
	})

	if err != nil {
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
	out, err := ddb.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String("transactions"),
		IndexName:              aws.String("locked_by_index"),
		KeyConditionExpression: aws.String(fmt.Sprintf("locked_by = %s", lockedByAttributeKey)),
		FilterExpression:       aws.String(fmt.Sprintf("tx_status < %s", txStatusAttributeKey)),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			lockedByAttributeKey: &types.AttributeValueMemberS{Value: lockedByNone},
			txStatusAttributeKey: &types.AttributeValueMemberN{Value: strconv.Itoa(int(metamorph_api.Status_MINED))},
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

		err = ddb.setLocked(ctx, transaction.Hash)
		if err != nil {
			return err
		}
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
	_, err := ddb.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String("transactions"),
		Key: map[string]types.AttributeValue{
			"tx_hash": &types.AttributeValueMemberB{Value: hash.CloneBytes()},
		},
		UpdateExpression: aws.String(fmt.Sprintf("SET tx_status = %s, reject_reason = %s", txStatusAttributeKey, rejectReasonAttributeKey)),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			txStatusAttributeKey:     &types.AttributeValueMemberN{Value: strconv.Itoa(int(status))},
			rejectReasonAttributeKey: &types.AttributeValueMemberS{Value: rejectReason},
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
	_, err := ddb.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String("transactions"),
		Key: map[string]types.AttributeValue{
			"tx_hash": &types.AttributeValueMemberB{Value: hash.CloneBytes()},
		},
		UpdateExpression: aws.String(fmt.Sprintf("SET block_height = %s, block_hash = %s, tx_status = %s", blockHeightAttributeKey, blockHashAttributeKey, txStatusAttributeKey)),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			blockHashAttributeKey:   &types.AttributeValueMemberB{Value: blockHash.CloneBytes()},
			blockHeightAttributeKey: &types.AttributeValueMemberN{Value: strconv.Itoa(int(blockHeight))},
			txStatusAttributeKey:    &types.AttributeValueMemberN{Value: strconv.Itoa(int(metamorph_api.Status_MINED))},
		},
	})

	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return err
	}

	return nil
}

// BlockItem marshal input value for new entry
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

	response, err := ddb.client.GetItem(ctx, &dynamodb.GetItemInput{
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
		processedAtTime, err = time.Parse(time.RFC3339, blockItem.ProcessedAt)
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

	blockItem := BlockItem{Hash: blockHash.CloneBytes(), ProcessedAt: time.Now().UTC().Format(time.RFC3339)}
	item, err := attributevalue.MarshalMap(blockItem)
	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return err
	}

	// put item into table
	_, err = ddb.client.PutItem(ctx, &dynamodb.PutItemInput{
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
