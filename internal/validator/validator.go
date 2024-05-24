package validator

import (
	"github.com/bitcoin-sv/arc/internal/beef"
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
	// ValidateEFTransaction Please note that bt.Tx should have all the fields of each input populated.
	ValidateEFTransaction(tx *bt.Tx, skipFeeValidation bool, skipScriptValidation bool) error
	ValidateBeef(beef *beef.BEEF, skipFeeValidation bool, skipScriptValidation bool) error
	IsExtended(tx *bt.Tx) bool
	IsBeef(txHex []byte) bool
}
