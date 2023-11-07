package metamorph

import (
	"github.com/bitcoin-sv/arc/metamorph/store"
	"github.com/bitcoin-sv/arc/metamorph/store/dynamodb"
	"github.com/bitcoin-sv/arc/metamorph/store/sql"
)

const (
	DbModeDynamoDB = "dynamodb"
)

func NewStore(dbMode string) (s store.MetamorphStore, err error) {
	switch dbMode {
	case DbModeDynamoDB:
		s, err = dynamodb.New()
	default:
		s, err = sql.New(dbMode)
	}

	return s, err
}
