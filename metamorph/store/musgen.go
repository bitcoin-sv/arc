//go:build ignore

package main

import (
	"reflect"

	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/ymz-ncnk/musgo"
)

func main() {
	musGo, err := musgo.New()
	if err != nil {
		panic(err)
	}
	// You should "Generate" for all involved custom types.
	unsafe := false // to generate safe code

	// reflect.Type could be created without the explicit variable.
	err = musGo.Generate(reflect.TypeOf((*store.StoreData)(nil)).Elem(), unsafe)
	if err != nil {
		panic(err)
	}
}
