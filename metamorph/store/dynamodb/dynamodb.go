package dynamodb

import (
	"context"
	"fmt"
	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/metamorph/store"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
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
	dateSinceKey                 = ":date_since"
	blockHeightAttributeKey      = ":block_height"
	blockHashAttributeKey        = ":block_hash"
	rejectReasonAttributeKey     = ":reject_reason"
	announcedAtAttributeKey      = ":announced_at"
	minedAtAttributeKey          = ":mined_at"
	merklePathAttributeKey       = ":merkle_path"
)

type DynamoDB struct {
	client                *dynamodb.Client
	hostname              string
	ttl                   time.Duration
	now                   func() time.Time
	transactionsTableName string
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
			},
		},
		TableName:   aws.String(ddb.transactionsTableName),
		BillingMode: types.BillingModePayPerRequest,
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
func (ddb *DynamoDB) SetUnlockedByName(ctx context.Context, lockedBy string) (int64, error) {
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

	numberOfUpdated := int64(0)

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

func (ddb *DynamoDB) GetUnmined(ctx context.Context, since time.Time, limit int64) ([]*store.StoreData, error) {
	// setup log and tracing
	startNanos := ddb.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_dynamodb").NewStat("getunmined").AddTime(startNanos)
	}()
	span, ctx := opentracing.StartSpanFromContext(ctx, "dynamodb:GetUnmined")
	defer span.Finish()

	// get unmined set
	out, err := ddb.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(ddb.transactionsTableName),
		IndexName:              aws.String("locked_by_index"),
		KeyConditionExpression: aws.String(fmt.Sprintf("locked_by = %s", lockedByAttributeKey)),
		FilterExpression:       aws.String(fmt.Sprintf("(tx_status < %s or tx_status = %s) and stored_at >= %s", txStatusAttributeKey, txStatusAttributeKeyOrphaned, dateSinceKey)),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			lockedByAttributeKey:         &types.AttributeValueMemberS{Value: lockedByNone},
			txStatusAttributeKey:         &types.AttributeValueMemberN{Value: strconv.Itoa(int(metamorph_api.Status_MINED))},
			txStatusAttributeKeyOrphaned: &types.AttributeValueMemberN{Value: strconv.Itoa(int(metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL))},
			dateSinceKey:                 &types.AttributeValueMemberS{Value: since.Format(time.DateOnly)},
		},
		Limit: aws.Int32(int32(limit)),
	})
	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return nil, err
	}

	data := make([]*store.StoreData, len(out.Items))

	for i, item := range out.Items {
		var transaction store.StoreData
		err = attributevalue.UnmarshalMap(item, &transaction)
		if err != nil {
			span.SetTag(string(ext.Error), true)
			span.LogFields(log.Error(err))
			return nil, err
		}

		data[i] = &transaction

		err = ddb.setLockedBy(ctx, transaction.Hash, ddb.hostname)
		if err != nil {
			span.SetTag(string(ext.Error), true)
			span.LogFields(log.Error(err))
			return nil, err
		}
	}

	return data, nil
}

func (ddb *DynamoDB) UpdateStatus(ctx context.Context, hash *chainhash.Hash, status metamorph_api.Status, rejectReason string) error {
	// setup log and tracing
	startNanos := ddb.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("UpdateStatus").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:UpdateStatus")
	defer span.Finish()

	// do not store other statuses than the following
	if status != metamorph_api.Status_REJECTED &&
		status != metamorph_api.Status_SEEN_ON_NETWORK &&
		status != metamorph_api.Status_MINED &&
		status != metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL {
		return nil
	}

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

func (ddb *DynamoDB) UpdateMined(ctx context.Context, txsBlocks *blocktx_api.TransactionBlocks) ([]*store.StoreData, error) {
	// setup log and tracing
	startNanos := ddb.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_dynamodb").NewStat("UpdateMined").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "dynamodb:UpdateMined")
	defer span.Finish()

	var storeData []*store.StoreData

	for _, txBlock := range txsBlocks.TransactionBlocks {
		// set block parameters and status - mined
		output, err := ddb.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName: aws.String(ddb.transactionsTableName),
			Key: map[string]types.AttributeValue{
				"tx_hash": &types.AttributeValueMemberB{Value: txBlock.TransactionHash},
			},
			UpdateExpression: aws.String(fmt.Sprintf("SET block_height = %s, block_hash = %s, tx_status = %s, mined_at = %s, merkle_path = %s", blockHeightAttributeKey, blockHashAttributeKey, txStatusAttributeKey, minedAtAttributeKey, merklePathAttributeKey)),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				blockHashAttributeKey:   &types.AttributeValueMemberB{Value: txBlock.BlockHash},
				blockHeightAttributeKey: &types.AttributeValueMemberN{Value: strconv.Itoa(int(txBlock.BlockHeight))},
				txStatusAttributeKey:    &types.AttributeValueMemberN{Value: strconv.Itoa(int(metamorph_api.Status_MINED))},
				minedAtAttributeKey:     &types.AttributeValueMemberS{Value: ddb.now().Format(time.RFC3339)},
				merklePathAttributeKey:  &types.AttributeValueMemberS{Value: txBlock.MerklePath},
			},
			ReturnValues: types.ReturnValueAllNew,
		})
		if err != nil {
			span.SetTag(string(ext.Error), true)
			span.LogFields(log.Error(err))
			return nil, err
		}

		var data store.StoreData
		err = attributevalue.UnmarshalMap(output.Attributes, &data)
		if err != nil {
			span.SetTag(string(ext.Error), true)
			span.LogFields(log.Error(err))
			return nil, err
		}

		storeData = append(storeData, &data)
	}

	return storeData, nil
}

// BlockItem marshal input value for new entry
type BlockItem struct {
	Hash        []byte `dynamodbav:"block_hash"`
	ProcessedAt string `dynamodbav:"processed_at"`
	Ttl         int64  `dynamodbav:"ttl"`
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

func (ddb *DynamoDB) Ping(ctx context.Context) error {
	startNanos := ddb.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_dynamodb").NewStat("Ping").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "dynamodb:Ping")
	defer span.Finish()

	_, err := ddb.client.ListTables(ctx, &dynamodb.ListTablesInput{})
	if err != nil {
		return err
	}

	return nil
}

func (p *DynamoDB) ClearData(ctx context.Context, retentionDays int32) (int64, error) {
	// Implementation not needed as clearing data handled by TTL-feature
	return 0, nil
}
