package main

import (
	"context"
	"fmt"
	"os"

	"github.com/TAAL-GmbH/arc/metamorph_api"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	argsWithoutProg := os.Args[1:]
	var txid string
	if len(argsWithoutProg) == 0 {
		panic("Missing txid")
	}
	txid = argsWithoutProg[0]

	ctx := context.Background()

	cc, err := grpc.DialContext(ctx, "localhost:8000", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	client := metamorph_api.NewMetaMorphAPIClient(cc)

	var res *metamorph_api.TransactionStatus
	res, err = client.GetTransactionStatus(ctx, &metamorph_api.TransactionStatusRequest{
		Txid: txid,
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("res %s: %#v\n", txid, res)
}
