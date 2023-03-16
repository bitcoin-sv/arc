package blocktx

import (
	"fmt"

	"github.com/TAAL-GmbH/arc/blocktx/store"
	"github.com/TAAL-GmbH/arc/blocktx/store/sql"
	"github.com/TAAL-GmbH/arc/blocktx/store/surrealdb"
)

func NewStore(dbMode string) (store.Interface, error) {
	if dbMode == "surrealdb" {
		return surrealdb.New("ws://localhost:8000/rpc")
	} else {
		blockStore, err := sql.New(dbMode)
		if err != nil {
			return nil, fmt.Errorf("could not connect to fn: %s", err.Error())
		}
		return blockStore, nil
	}
}
