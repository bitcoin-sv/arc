package callbacker

import (
	"fmt"
	"path"
	"path/filepath"
	"time"

	"github.com/TAAL-GmbH/arc/callbacker/store"
	callbackerBadgerhold "github.com/TAAL-GmbH/arc/callbacker/store/badgerhold"
)

func NewStore(folder string) (store.Store, error) {
	f, err := filepath.Abs(path.Join(folder, "callbacker"))
	if err != nil {
		return nil, fmt.Errorf("could not get absolute path: %v", err)
	}

	callbackStore, err := callbackerBadgerhold.New(f, 2*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("could not open callbacker store: %v", err)
	}

	return callbackStore, nil
}
