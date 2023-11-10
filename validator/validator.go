package validator

import (
	"github.com/libsv/go-bt/v2"
)

type Outpoint struct {
	Txid string
	Idx  uint32
}

type OutpointData struct {
	ScriptPubKey []byte
	Satoshis     int64
}

type Validator interface {
	// ValidateTransaction Please note that bt.Tx should have all the fields of each input populated.
	ValidateTransaction(tx *bt.Tx, skipFeeValidation bool, skipScriptValidation bool) error
	IsExtended(tx *bt.Tx) bool
}
