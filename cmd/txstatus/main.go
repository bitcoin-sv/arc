package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/tracing"
	"github.com/ordishs/gocore"
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

	addresses, found := gocore.Config().Get("metamorphAddresses")
	if !found {
		panic("Missing metamorphAddresses")
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`), // This sets the initial balancing policy.
	}

	cc, err := grpc.DialContext(ctx, addresses, tracing.AddGRPCDialOptions(opts)...)
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

	var b []byte
	b, err = json.Marshal(res)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%s\n", b)
}
