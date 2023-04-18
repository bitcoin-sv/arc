package blocktx

import (
	"fmt"

	"github.com/bitcoin-sv/arc/blocktx/store"
	"github.com/bitcoin-sv/arc/blocktx/store/sql"
)

func NewStore(dbMode string) (store.Interface, error) {
	blockStore, err := sql.New(dbMode)
	if err != nil {
		return nil, fmt.Errorf("could not connect to fn: %s", err.Error())
	}

	return blockStore, nil
}
