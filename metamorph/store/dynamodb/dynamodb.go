package dynamodb

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/metamorph/store"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

type DynamoDB struct {
	dynamoCli *dynamodb.Client
}

const ISO8601 = "2006-01-02T15:04:05.999Z"

func New() (store.MetamorphStore, error) {
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
	} else if !exists {
		dynamodbClient.CreateTransactionsTable()
	}

	// create table if not exists
	fmt.Println("aaaaaa")
	exists, err = dynamodbClient.TableExists("blocks")
	if err != nil {
		fmt.Println(err)
		return dynamodbClient, err
	} else if !exists {
		fmt.Println("creating ....")
		dynamodbClient.CreateBlocksTable()
	}

	// Using the Config value, create the DynamoDB client
	return dynamodbClient, nil

}

// CREATE TABLE IF NOT EXISTS transactions (
// 	hash BLOB PRIMARY KEY,
// 	stored_at TEXT,
// 	announced_at TEXT,
// 	mined_at TEXT,
// 	status INTEGER,
// 	block_height BIGINT,
// 	block_hash BLOB,
// 	callback_url TEXT,
// 	callback_token TEXT,
// 	merkle_proof TEXT,
// 	reject_reason TEXT,
// 	raw_tx BLOB
// 	);

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
	if _, err := ddb.dynamoCli.CreateTable(context.TODO(), &dynamodb.CreateTableInput{
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("hash"),
				AttributeType: types.ScalarAttributeTypeB,
			},
		},
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("hash"),
				KeyType:       types.KeyTypeHash,
			}},
		TableName: aws.String("transactions"),
		ProvisionedThroughput: &types.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(10),
			WriteCapacityUnits: aws.Int64(10),
		},
	}); err != nil {
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
				AttributeName: aws.String("hash"),
				AttributeType: types.ScalarAttributeTypeB,
			},
		},
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("hash"),
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

func (dynamodb *DynamoDB) Get(ctx context.Context, key []byte) (*store.StoreData, error) {
	return nil, nil
}

func (dynamodb *DynamoDB) Set(ctx context.Context, key []byte, value *store.StoreData) error {
	// 	startNanos := time.Now().UnixNano()
	// 	defer func() {
	// 		gocore.NewStat("mtm_store_sql").NewStat("Set").AddTime(startNanos)
	// 	}()
	// 	span, _ := opentracing.StartSpanFromContext(ctx, "sql:Set")
	// 	defer span.Finish()

	// 	q := `INSERT INTO transactions (
	// 		 stored_at
	// 		,announced_at
	// 		,mined_at
	// 		,hash
	// 		,status
	// 		,block_height
	// 		,block_hash
	// 		,callback_url
	// 		,callback_token
	// 		,merkle_proof
	// 		,reject_reason
	// 		,raw_tx
	// 	) VALUES (
	// 		 $1
	// 		,$2
	// 		,$3
	// 		,$4
	// 		,$5
	// 		,$6
	// 		,$7
	// 		,$8
	// 		,$9
	// 		,$10
	// 		,$11
	// 		,$12
	// 	);`

	// 	var storedAt string
	// 	var announcedAt string
	// 	var minedAt string
	// 	var txHash []byte
	// 	var blockHash []byte

	// 	if value.Hash != nil {
	// 		txHash = value.Hash.CloneBytes()
	// 	}

	// 	if value.BlockHash != nil {
	// 		blockHash = value.BlockHash.CloneBytes()
	// 	}

	// 	if value.StoredAt.UnixMilli() != 0 {
	// 		storedAt = value.StoredAt.UTC().Format(ISO8601)
	// 	}

	// 	// If the storedAt time is zero, set it to now on insert
	// 	if value.StoredAt.IsZero() {
	// 		value.StoredAt = time.Now()
	// 	}

	// 	if value.AnnouncedAt.UnixMilli() != 0 {
	// 		announcedAt = value.AnnouncedAt.UTC().Format(ISO8601)
	// 	}

	// 	if value.MinedAt.UnixMilli() != 0 {
	// 		minedAt = value.MinedAt.UTC().Format(ISO8601)
	// 	}

	// 	_, err := s.db.ExecContext(ctx, q,
	// 		storedAt,
	// 		announcedAt,
	// 		minedAt,
	// 		txHash,
	// 		value.Status,
	// 		value.BlockHeight,
	// 		blockHash,
	// 		value.CallbackUrl,
	// 		value.CallbackToken,
	// 		value.MerkleProof,
	// 		value.RejectReason,
	// 		value.RawTx,
	// 	)

	// 	if err != nil {
	// 		span.SetTag(string(ext.Error), true)
	// 		span.LogFields(log.Error(err))
	// 	}

	// 	return err
	// }

	// func (s *SQL) GetUnmined(ctx context.Context, callback func(s *store.StoreData)) error {
	// 	startNanos := time.Now().UnixNano()
	// 	defer func() {
	// 		gocore.NewStat("mtm_store_sql").NewStat("getunmined").AddTime(startNanos)
	// 	}()
	// 	span, _ := opentracing.StartSpanFromContext(ctx, "sql:GetUnmined")
	// 	defer span.Finish()

	// 	q := `SELECT
	// 	   stored_at
	// 		,announced_at
	// 		,mined_at
	// 		,hash
	// 		,status
	// 		,block_height
	// 		,block_hash
	// 		,callback_url
	// 		,callback_token
	// 		,merkle_proof
	// 		,raw_tx
	// 	 	FROM transactions WHERE status < $1;`

	// 	rows, err := s.db.QueryContext(ctx, q, metamorph_api.Status_MINED)
	// 	if err != nil {
	// 		span.SetTag(string(ext.Error), true)
	// 		span.LogFields(log.Error(err))
	// 		return err
	// 	}
	// 	defer rows.Close()

	// 	for rows.Next() {
	// 		data := &store.StoreData{}

	// 		var txHash []byte
	// 		var blockHash []byte
	// 		var storedAt string
	// 		var announcedAt string
	// 		var minedAt string

	// 		if err = rows.Scan(
	// 			&storedAt,
	// 			&announcedAt,
	// 			&minedAt,
	// 			&txHash,
	// 			&data.Status,
	// 			&data.BlockHeight,
	// 			&blockHash,
	// 			&data.CallbackUrl,
	// 			&data.CallbackToken,
	// 			&data.MerkleProof,
	// 			&data.RawTx,
	// 		); err != nil {
	// 			return err
	// 		}

	// 		if txHash != nil {
	// 			data.Hash, _ = chainhash.NewHash(txHash)
	// 		}

	// 		if blockHash != nil {
	// 			data.BlockHash, _ = chainhash.NewHash(blockHash)
	// 		}

	// 		if storedAt != "" {
	// 			data.StoredAt, err = time.Parse(ISO8601, storedAt)
	// 			if err != nil {
	// 				span.SetTag(string(ext.Error), true)
	// 				span.LogFields(log.Error(err))
	// 				return err
	// 			}
	// 		}

	// 		if announcedAt != "" {
	// 			data.AnnouncedAt, err = time.Parse(ISO8601, announcedAt)
	// 			if err != nil {
	// 				span.SetTag(string(ext.Error), true)
	// 				span.LogFields(log.Error(err))
	// 				return err
	// 			}
	// 		}
	// 		if minedAt != "" {
	// 			data.MinedAt, err = time.Parse(ISO8601, minedAt)
	// 			if err != nil {
	// 				span.SetTag(string(ext.Error), true)
	// 				span.LogFields(log.Error(err))
	// 				return err
	// 			}
	// 		}

	// 		callback(data)
	// 	}

	return nil
}

func (dynamodb *DynamoDB) Del(ctx context.Context, key []byte) error {
	return nil
}

func (dynamodb *DynamoDB) GetUnmined(_ context.Context, callback func(s *store.StoreData)) error {
	return nil
}

func (dynamodb *DynamoDB) UpdateStatus(ctx context.Context, hash *chainhash.Hash, status metamorph_api.Status, rejectReason string) error {
	return nil
}

func (dynamodb *DynamoDB) UpdateMined(ctx context.Context, hash *chainhash.Hash, blockHash *chainhash.Hash, blockHeight uint64) error {
	return nil
}

func (dynamodb *DynamoDB) Close(ctx context.Context) error {
	return nil
}

func (dynamodb *DynamoDB) GetBlockProcessed(ctx context.Context, blockHash *chainhash.Hash) (*time.Time, error) {
	return nil, nil
}

func (dynamodb *DynamoDB) SetBlockProcessed(ctx context.Context, blockHash *chainhash.Hash) error {
	return nil
}
