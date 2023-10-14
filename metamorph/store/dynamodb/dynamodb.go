package dynamodb

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
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

	// Using the Config value, create the DynamoDB client
	return &DynamoDB{dynamoCli: dynamodb.NewFromConfig(cfg)}, nil

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
