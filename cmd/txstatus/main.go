package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/TAAL-GmbH/arc/api/transactionHandler"
	"github.com/TAAL-GmbH/arc/blocktx"
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

	var b []byte
	b, err = json.Marshal(res)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%s\n", b)
}
