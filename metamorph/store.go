package metamorph

import (
	"path"

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
		s, err = dynamodb.New()
	default:
		s, err = sql.New(dbMode)
	}

	return s, err
}
