package callbacker

import (
	"fmt"
	"path"
	"path/filepath"
	"time"

	"github.com/bitcoin-sv/arc/callbacker/store"
	callbackerBadgerhold "github.com/bitcoin-sv/arc/callbacker/store/badgerhold"
)

func NewStore(folder string, interval time.Duration) (store.Store, error) {
	f, err := filepath.Abs(path.Join(folder, "callbacker"))
	if err != nil {
		return nil, fmt.Errorf("could not get absolute path: %v", err)
	}

	callbackStore, err := callbackerBadgerhold.New(f, interval)
	if err != nil {
		return nil, fmt.Errorf("could not open callbacker store: %v", err)
	}

	return callbackStore, nil
}
