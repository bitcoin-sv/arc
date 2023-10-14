package dynamodb

import (
	"context"
	"log"
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

func New() (store.MetamorphStore, error) {
	// Using the SDK's default configuration, loading additional config
	// and credentials values from the environment variables, shared
	// credentials, and shared configuration files
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(os.Getenv("AWS_REGION")))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
		return &DynamoDB{}, err
	}

	dynamodbClient := &DynamoDB{dynamoCli: dynamodb.NewFromConfig(cfg)}
	dynamodbClient.CreateTransactionsTable()

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
		log.Fatalf("failed to list tables, %v", err)
		return false, err
	}

	for _, tableName := range resp.TableNames {
		if tableName == "transactions" {
			return true, nil
		}
	}

	return false, nil
}

// CreateTransactionsTable creates a DynamoDB table for storing metamorph user transactions
func (ddb *DynamoDB) CreateTransactionsTable() error {
	// make sure table doesn't exist yet
	exists, err := ddb.TableExists("transactions")
	if err != nil {
		return err
	} else if exists {
		log.Printf("Table 'transactions' already exists")
		return nil
	}

	// construct primary key and create table
	_, err = ddb.dynamoCli.CreateTable(context.TODO(), &dynamodb.CreateTableInput{
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
	})
	if err != nil {
		log.Printf("Couldn't create table %v. Here's why: %v\n", "transactions", err)
		return err
	} else {
		log.Printf("table 'transactions' was created successfully")
	}
	return nil
}

func (dynamodb *DynamoDB) Get(ctx context.Context, key []byte) (*store.StoreData, error) {
	return nil, nil
}

func (dynamodb *DynamoDB) Set(ctx context.Context, key []byte, value *store.StoreData) error {
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
