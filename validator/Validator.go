package validator

import "github.com/libsv/go-bt/v2"

type Validator interface {
	// Please note that bt.Tx should have all the fields of each input populated.
	// This is automatic
	ValidateTransaction(tx *bt.Tx) error
}
