package blocktx

import (
	"fmt"

	"github.com/TAAL-GmbH/arc/blocktx/store"
	"github.com/TAAL-GmbH/arc/blocktx/store/sql"
)

func NewStore(dbMode string) (store.Interface, error) {
	blockStore, err := sql.New(dbMode)
	if err != nil {
		return nil, fmt.Errorf("could not connect to fn: %s", err.Error())
	}

	return blockStore, nil
}
