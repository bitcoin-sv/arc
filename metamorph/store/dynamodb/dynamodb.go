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

	lockedByAttributeKey         = ":locked_by"
	txStatusAttributeKey         = ":tx_status"
	txStatusAttributeKeyOrphaned = ":tx_orphaned"
	blockHeightAttributeKey      = ":block_height"
	blockHashAttributeKey        = ":block_hash"
	rejectReasonAttributeKey     = ":reject_reason"
	announcedAtAttributeKey      = ":announced_at"
	minedAtAttributeKey          = ":mined_at"
)

type DynamoDB struct {
	client                *dynamodb.Client
	hostname              string
	ttl                   time.Duration
	now                   func() time.Time
	transactionsTableName string
	blocksTableName       string
}

func WithNow(nowFunc func() time.Time) func(*DynamoDB) {
	return func(p *DynamoDB) {
		p.now = nowFunc
	}
}

type Option func(f *DynamoDB)

func New(client *dynamodb.Client, hostname string, timeToLive time.Duration, tableNameSuffix string, opts ...Option) (*DynamoDB, error) {
	repo := &DynamoDB{
		client:                client,
		hostname:              hostname,
		ttl:                   timeToLive,
		now:                   time.Now,
		transactionsTableName: fmt.Sprintf("transactions-%s", tableNameSuffix),
		blocksTableName:       fmt.Sprintf("blocks-%s", tableNameSuffix),
	}

	// apply options
	for _, opt := range opts {
		opt(repo)
	}

	err := repo.initialize(context.Background(), repo)

	if err != nil {
		return nil, err
	}
	return repo, nil
}

func (ddb *DynamoDB) initialize(ctx context.Context, dynamodbClient *DynamoDB) error {

	// create table if not exists
	exists, err := dynamodbClient.TableExists(ctx, ddb.transactionsTableName)
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
	exists, err = dynamodbClient.TableExists(ctx, ddb.blocksTableName)
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
		TableName: aws.String(ddb.transactionsTableName),
		ProvisionedThroughput: &types.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(10),
			WriteCapacityUnits: aws.Int64(10),
		},
	})
	if err != nil {
		return err
	}

	ttlInput := dynamodb.UpdateTimeToLiveInput{
		TableName: aws.String(ddb.transactionsTableName),
		TimeToLiveSpecification: &types.TimeToLiveSpecification{
			Enabled:       aws.Bool(true),
			AttributeName: aws.String("ttl"),
		},
	}
	_, err = ddb.client.UpdateTimeToLive(ctx, &ttlInput)
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
			},
		},
		TableName: aws.String(ddb.blocksTableName),
		ProvisionedThroughput: &types.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(10),
			WriteCapacityUnits: aws.Int64(10),
		},
	}); err != nil {
		return err
	}

	ttlInput := &dynamodb.UpdateTimeToLiveInput{
		TableName: aws.String(ddb.blocksTableName),
		TimeToLiveSpecification: &types.TimeToLiveSpecification{
			Enabled:       aws.Bool(true),
			AttributeName: aws.String("ttl"),
		},
	}
	_, err := ddb.client.UpdateTimeToLive(ctx, ttlInput)
	if err != nil {
		return err
	}

	return nil
}

func (ddb *DynamoDB) IsCentralised() bool {
	return true
}

func (ddb *DynamoDB) Get(ctx context.Context, key []byte) (*store.StoreData, error) {
	// config log and tracing
	startNanos := ddb.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_dynamodb").NewStat("Get").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "dynamodb:Get")
	defer span.Finish()

	val := map[string]types.AttributeValue{
		"tx_hash": &types.AttributeValueMemberB{Value: key},
	}

	response, err := ddb.client.GetItem(ctx, &dynamodb.GetItemInput{
		Key: val, TableName: aws.String(ddb.transactionsTableName),
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
	startNanos := ddb.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_dynamodb").NewStat("Set").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "dynamodb:Set")
	defer span.Finish()

	ttl := ddb.now().Add(ddb.ttl)
	value.Ttl = ttl.Unix()
	value.LockedBy = ddb.hostname
	value.StoredAt = ddb.now()

	// marshal input value for new entry
	item, err := attributevalue.MarshalMap(value)
	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return err
	}

	// put item into table
	_, err = ddb.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(ddb.transactionsTableName), Item: item,
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
	startNanos := ddb.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_dynamodb").NewStat("Del").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "dynamodb:Del")
	defer span.Finish()

	val := map[string]types.AttributeValue{
		"tx_hash": &types.AttributeValueMemberB{Value: key},
	}

	_, err := ddb.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(ddb.transactionsTableName), Key: val,
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
	startNanos := ddb.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("UpdateStatus").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:UpdateStatus")
	defer span.Finish()

	// update tx
	for _, hash := range hashes {

		err := ddb.setLockedBy(ctx, hash, lockedByNone)
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
	startNanos := ddb.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("UpdateStatus").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:UpdateStatus")
	defer span.Finish()

	out, err := ddb.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(ddb.transactionsTableName),
		IndexName:              aws.String("locked_by_index"),
		KeyConditionExpression: aws.String(fmt.Sprintf("locked_by = %s", lockedByAttributeKey)),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			lockedByAttributeKey: &types.AttributeValueMemberS{Value: lockedBy},
		},
	})
	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return 0, err
	}

	numberOfUpdated := 0

	for _, item := range out.Items {
		var transaction store.StoreData
		err = attributevalue.UnmarshalMap(item, &transaction)
		if err != nil {
			span.SetTag(string(ext.Error), true)
			span.LogFields(log.Error(err))
			return 0, err
		}

		if transaction.LockedBy == lockedByNone {
			continue
		}

		err = ddb.setLockedBy(ctx, transaction.Hash, lockedByNone)
		if err != nil {
			span.SetTag(string(ext.Error), true)
			span.LogFields(log.Error(err))
			return numberOfUpdated, err
		}

		numberOfUpdated++
	}

	return numberOfUpdated, nil
}

func (ddb *DynamoDB) setLockedBy(ctx context.Context, hash *chainhash.Hash, lockedByValue string) error {
	_, err := ddb.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(ddb.transactionsTableName),
		Key: map[string]types.AttributeValue{
			"tx_hash": &types.AttributeValueMemberB{Value: hash.CloneBytes()},
		},
		UpdateExpression: aws.String(fmt.Sprintf("SET locked_by = %s", lockedByAttributeKey)),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			lockedByAttributeKey: &types.AttributeValueMemberS{Value: lockedByValue},
		},
	})

	if err != nil {
		return err
	}

	return nil
}

func (ddb *DynamoDB) GetUnmined(ctx context.Context, callback func(s *store.StoreData)) error {
	// setup log and tracing
	startNanos := ddb.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_dynamodb").NewStat("getunmined").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "dynamodb:GetUnmined")
	defer span.Finish()

	// get unmined set
	out, err := ddb.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(ddb.transactionsTableName),
		IndexName:              aws.String("locked_by_index"),
		KeyConditionExpression: aws.String(fmt.Sprintf("locked_by = %s", lockedByAttributeKey)),
		FilterExpression:       aws.String(fmt.Sprintf("tx_status < %s or tx_status = %s", txStatusAttributeKey, txStatusAttributeKeyOrphaned)),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			lockedByAttributeKey:         &types.AttributeValueMemberS{Value: lockedByNone},
			txStatusAttributeKey:         &types.AttributeValueMemberN{Value: strconv.Itoa(int(metamorph_api.Status_MINED))},
			txStatusAttributeKeyOrphaned: &types.AttributeValueMemberN{Value: strconv.Itoa(int(metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL))},
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

		err = ddb.setLockedBy(ctx, transaction.Hash, ddb.hostname)
		if err != nil {
			span.SetTag(string(ext.Error), true)
			span.LogFields(log.Error(err))
			return err
		}
	}
	return nil
}

func (ddb *DynamoDB) UpdateStatus(ctx context.Context, hash *chainhash.Hash, status metamorph_api.Status, rejectReason string) error {
	// setup log and tracing
	startNanos := ddb.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("UpdateStatus").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:UpdateStatus")
	defer span.Finish()

	updateExpression := fmt.Sprintf("SET tx_status = %s, reject_reason = %s", txStatusAttributeKey, rejectReasonAttributeKey)
	expressionAttributevalues := map[string]types.AttributeValue{
		txStatusAttributeKey:     &types.AttributeValueMemberN{Value: strconv.Itoa(int(status))},
		rejectReasonAttributeKey: &types.AttributeValueMemberS{Value: rejectReason},
	}

	switch status {
	case metamorph_api.Status_ANNOUNCED_TO_NETWORK:
		updateExpression = updateExpression + fmt.Sprintf(", announced_at = %s", announcedAtAttributeKey)
		expressionAttributevalues[announcedAtAttributeKey] = &types.AttributeValueMemberS{Value: ddb.now().Format(time.RFC3339)}
	case metamorph_api.Status_MINED:
		updateExpression = updateExpression + fmt.Sprintf(", mined_at = %s", minedAtAttributeKey)
		expressionAttributevalues[minedAtAttributeKey] = &types.AttributeValueMemberS{Value: ddb.now().Format(time.RFC3339)}
	}

	// update tx
	_, err := ddb.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(ddb.transactionsTableName),
		Key: map[string]types.AttributeValue{
			"tx_hash": &types.AttributeValueMemberB{Value: hash.CloneBytes()},
		},
		UpdateExpression:          aws.String(updateExpression),
		ExpressionAttributeValues: expressionAttributevalues,
	})

	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return err
	}

	return nil
}

func (ddb *DynamoDB) RemoveCallbacker(ctx context.Context, hash *chainhash.Hash) error {
	// setup log and tracing
	startNanos := ddb.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("RemoveCallbacker").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:RemoveCallbacker")
	defer span.Finish()

	updateExpression := "SET callback_url = ''"

	// update tx
	_, err := ddb.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(ddb.transactionsTableName),
		Key: map[string]types.AttributeValue{
			"tx_hash": &types.AttributeValueMemberB{Value: hash.CloneBytes()},
		},
		UpdateExpression: aws.String(updateExpression),
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
	startNanos := ddb.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_dynamodb").NewStat("UpdateMined").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "dynamodb:UpdateMined")
	defer span.Finish()

	// set block parameters and status - mined
	_, err := ddb.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(ddb.transactionsTableName),
		Key: map[string]types.AttributeValue{
			"tx_hash": &types.AttributeValueMemberB{Value: hash.CloneBytes()},
		},
		UpdateExpression: aws.String(fmt.Sprintf("SET block_height = %s, block_hash = %s, tx_status = %s, mined_at = %s", blockHeightAttributeKey, blockHashAttributeKey, txStatusAttributeKey, minedAtAttributeKey)),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			blockHashAttributeKey:   &types.AttributeValueMemberB{Value: blockHash.CloneBytes()},
			blockHeightAttributeKey: &types.AttributeValueMemberN{Value: strconv.Itoa(int(blockHeight))},
			txStatusAttributeKey:    &types.AttributeValueMemberN{Value: strconv.Itoa(int(metamorph_api.Status_MINED))},
			minedAtAttributeKey:     &types.AttributeValueMemberS{Value: ddb.now().Format(time.RFC3339)},
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
	Ttl         int64  `dynamodbav:"ttl"`
}

func (ddb *DynamoDB) GetBlockProcessed(ctx context.Context, blockHash *chainhash.Hash) (*time.Time, error) {
	// setup log and tracing
	startNanos := ddb.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_dynamodb").NewStat("GetBlockProcessed").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "dynamodb:GetBlockProcessed")
	defer span.Finish()

	val := map[string]types.AttributeValue{
		"block_hash": &types.AttributeValueMemberB{Value: blockHash.CloneBytes()},
	}

	response, err := ddb.client.GetItem(ctx, &dynamodb.GetItemInput{
		Key: val, TableName: aws.String(ddb.blocksTableName),
	})
	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return nil, err
	}

	if len(response.Item) != 0 {
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

	return nil, nil
}

func (ddb *DynamoDB) SetBlockProcessed(ctx context.Context, blockHash *chainhash.Hash) error {
	// set log and tracing
	startNanos := ddb.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_dynamodb").NewStat("SetBlockProcessed").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "dynamodb:SetBlockProcessed")
	defer span.Finish()

	ttl := ddb.now().Add(ddb.ttl)

	blockItem := BlockItem{
		Hash:        blockHash.CloneBytes(),
		ProcessedAt: ddb.now().UTC().Format(time.RFC3339),
		Ttl:         ttl.Unix(),
	}
	item, err := attributevalue.MarshalMap(blockItem)
	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return err
	}

	// put item into table
	_, err = ddb.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(ddb.blocksTableName), Item: item,
	})

	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return err
	}

	return nil
}

func (ddb *DynamoDB) Close(ctx context.Context) error {
	startNanos := ddb.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_dynamodb").NewStat("Close").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "dynamodb:Close")
	defer span.Finish()

	ctx.Done()
	return nil
}
