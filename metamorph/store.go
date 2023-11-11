package metamorph

import (
	"context"
	"os"
	"path"

	"github.com/aws/aws-sdk-go-v2/config"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/bitcoin-sv/arc/metamorph/store"
	"github.com/bitcoin-sv/arc/metamorph/store/badger"
	"github.com/bitcoin-sv/arc/metamorph/store/dynamodb"
	"github.com/bitcoin-sv/arc/metamorph/store/sql"
)

const (
	DbModeBadger   = "badger"
	DbModeDynamoDB = "dynamodb"
)

func NewStore(dbMode string, folder string) (s store.MetamorphStore, err error) {
	switch dbMode {
	case DbModeBadger:
		s, err = badger.New(path.Join(folder, "metamorph"))
	case DbModeDynamoDB:
		hostname, err := os.Hostname()
		if err != nil {
			return nil, err
		}

		ctx := context.Background()
		cfg, err := config.LoadDefaultConfig(ctx, config.WithEC2IMDSRegion())
		if err != nil {
			return nil, err
		}

		s, err = dynamodb.New(
			awsdynamodb.NewFromConfig(cfg),
			hostname,
		)
		if err != nil {
			return nil, err
		}
	default:
		s, err = sql.New(dbMode)
	}

	return s, err
}
