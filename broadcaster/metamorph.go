package broadcaster

import (
	"context"
	"fmt"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/tracing"
	"github.com/libsv/go-bt/v2"
	"github.com/ordishs/gocore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type MetamorphBroadcaster struct {
	address string
	client  metamorph_api.MetaMorphAPIClient
}

func NewMetamorphBroadcaster(address string) *MetamorphBroadcaster {
	addresses, _ := gocore.Config().Get("metamorphAddresses") //, "localhost:8000")
	fmt.Printf("Metamorph addresses: %s\n", addresses)

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`), // This sets the initial balancing policy.grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`), // This sets the initial balancing policy.
	}

	cc, err := grpc.DialContext(context.Background(), addresses, tracing.AddGRPCDialOptions(opts)...)
	if err != nil {
		panic(fmt.Errorf("DIALCONTEXT: %v", err))
	}

	client := metamorph_api.NewMetaMorphAPIClient(cc)

	return &MetamorphBroadcaster{
		address: address,
		client:  client,
	}
}

func (m *MetamorphBroadcaster) PutTransaction(ctx context.Context, tx *bt.Tx, waitFor metamorph_api.Status) (*metamorph_api.TransactionStatus, error) {
	return m.client.PutTransaction(ctx, &metamorph_api.TransactionRequest{
		RawTx:         tx.Bytes(),
		WaitForStatus: waitFor,
	})
}

func (m *MetamorphBroadcaster) GetTransactionStatus(ctx context.Context, txID string) (*metamorph_api.TransactionStatus, error) {
	return nil, nil
}
