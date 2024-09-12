package callbacker

import (
	"time"
)

type CallbackerI interface {
	Send(url, token string, callback *Callback) bool
	Health() error
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
