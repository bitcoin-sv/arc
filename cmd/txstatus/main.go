package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/bitcoin-sv/arc/api/transactionHandler"
	"github.com/bitcoin-sv/arc/blocktx"
	"github.com/ordishs/gocore"
)

var logger = gocore.Log("txstatus")

func main() {
	argsWithoutProg := os.Args[1:]
	var txid string
	if len(argsWithoutProg) == 0 {
		panic("Missing txid")
	}
	txid = argsWithoutProg[0]

	ctx := context.Background()

	addresses, found := gocore.Config().Get("metamorphAddresses")
	if !found {
		panic("Missing metamorphAddresses")
	}

	btxAddress, _ := gocore.Config().Get("blocktxAddress") //, "localhost:8001")
	bTx := blocktx.NewClient(logger, btxAddress)

	txHandler, err := transactionHandler.NewMetamorph(addresses, bTx)
	if err != nil {
		panic(err)
	}

	var res *transactionHandler.TransactionStatus
	res, err = txHandler.GetTransactionStatus(ctx, txid)
	if err != nil {
		panic(err)
	}

	type response struct {
		*transactionHandler.TransactionStatus
		TransactionBytes string    `json:"transactionBytes"`
		Timestamp        time.Time `json:"timestamp"`
	}

	transactionBytes, err := txHandler.GetTransaction(ctx, txid)
	if err != nil {
		panic(err)
	}

	r := &response{
		TransactionStatus: res,
		TransactionBytes:  hex.EncodeToString(transactionBytes),
		Timestamp:         time.Unix(res.Timestamp, 0).UTC(),
	}

	var b []byte
	b, err = json.MarshalIndent(r, "", "  ")
	if err != nil {
		panic(err)
	}

	fmt.Printf("%s\n", b)
}
