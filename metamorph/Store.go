package metamorph

import (
	"path"

	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/TAAL-GmbH/arc/metamorph/store/badger"
	"github.com/TAAL-GmbH/arc/metamorph/store/sql"
)

func NewStore(dbMode string, folder string) (s store.MetamorphStore, err error) {
	switch dbMode {
	case "badger":
		s, err = badger.New(path.Join(folder, "metamorph"))
	default:
		s, err = sql.New(dbMode)
	}

	return s, err
}
