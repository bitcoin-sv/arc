package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"os"

	"github.com/bitcoin-sv/arc/metamorph/store"
	"github.com/dgraph-io/badger/v3"
)

func main() {
	argsWithoutProg := os.Args[1:]
	var dir string
	if len(argsWithoutProg) == 0 {
		panic("Missing dir")
	}
	dir = argsWithoutProg[0]

	opts := badger.DefaultOptions(dir).
		WithLoggingLevel(badger.ERROR).WithNumMemtables(32)
	s, err := badger.Open(opts)
	if err != nil {
		panic(err)
	}

	var result store.StoreData

	err = s.View(func(tx *badger.Txn) error {
		iter := tx.NewIterator(badger.DefaultIteratorOptions)
		defer iter.Close()

		fmt.Println("[")
		n := 0
		for iter.Rewind(); iter.Valid(); iter.Next() {
			item := iter.Item()
			if item.IsDeletedOrExpired() {
				continue
			}

			if err = item.Value(func(val []byte) error {
				dec := gob.NewDecoder(bytes.NewReader(val))
				return dec.Decode(&result)
			}); err != nil {
				return fmt.Errorf("failed to decode data: %w", err)
			}

			var b []byte
			b, err = json.Marshal(result)
			if err != nil {
				return err
			}
			if n == 0 {
				fmt.Println(string(b))
			} else {
				fmt.Println("," + string(b))
			}
			n++
		}
		fmt.Println("]")

		return nil
	})

	if err != nil {
		panic(err)
	}
}
