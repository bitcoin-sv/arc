package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/tracing"
	"github.com/ordishs/gocore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

func main() {
	argsWithoutProg := os.Args[1:]
	var checkMetamorph string
	if len(argsWithoutProg) == 1 {
		checkMetamorph = argsWithoutProg[0]
	}

	ctx := context.Background()

	addresses, found := gocore.Config().Get("metamorphAddresses")
	if !found {
		panic("Missing metamorphAddresses")
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`), // This sets the initial balancing policy.
	}

	metamorphs := strings.Split(addresses, ",")
	if checkMetamorph != "" && strings.Contains(addresses, checkMetamorph) {
		health := getMetamorphHealth(ctx, checkMetamorph, opts)
		jsonRes, err := json.Marshal(health)
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s\n", string(jsonRes))
	} else {
		allRes := make(map[string]*metamorph_api.HealthResponse)
		for _, metamorph := range metamorphs {
			health := getMetamorphHealth(ctx, metamorph, opts)
			allRes[metamorph] = health
		}
		jsonRes, err := json.Marshal(allRes)
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s\n", string(jsonRes))
	}
}

func getMetamorphHealth(ctx context.Context, checkMetamorph string, opts []grpc.DialOption) *metamorph_api.HealthResponse {
	cc, err := grpc.DialContext(ctx, checkMetamorph, tracing.AddGRPCDialOptions(opts)...)
	if err != nil {
		panic(err)
	}

	client := metamorph_api.NewMetaMorphAPIClient(cc)

	var res *metamorph_api.HealthResponse
	res, err = client.Health(ctx, &emptypb.Empty{})
	if err != nil {
		panic(err)
	}

	return res
}
