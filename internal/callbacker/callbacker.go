package callbacker

import (
	"time"
)

type CallbackerI interface {
	Send(url, token string, callback *Callback) bool
	SendBatch(url, token string, callbacks []*Callback) bool
}

type Callback struct {
	Timestamp time.Time `json:"timestamp"`

	CompetingTxs []string `json:"competingTxs,omitempty"`

	TxID       string  `json:"txid"`
	TxStatus   string  `json:"txStatus"`
	ExtraInfo  *string `json:"extraInfo,omitempty"`
	MerklePath *string `json:"merklePath,omitempty"`

	BlockHash   *string `json:"blockHash,omitempty"`
	BlockHeight *uint64 `json:"blockHeight,omitempty"`
}

type BatchCallback struct {
	Count     int         `json:"count"`
	Callbacks []*Callback `json:"callbacks"`
}
