package validator

import "github.com/libsv/go-bt/v2"

type Outpoint struct {
	Txid string
	Idx  uint32
}

type OutpointData struct {
	ScriptPubKey []byte
	Satoshis     int64
}

type Validator interface {
	ValidateTransaction(tx *bt.Tx, parentData map[Outpoint]OutpointData) error
}
