package client

import "github.com/ordishs/go-bitcoin"

type Node interface {
	GetRawTransaction(txID string) (rawTx *bitcoin.RawTransaction, err error)
	SendRawTransaction(tx string) (txID string, err error)
}
