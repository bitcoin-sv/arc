package callbacker

import (
	"context"

	"github.com/TAAL-GmbH/arc/callbacker/callbacker_api"
	"github.com/TAAL-GmbH/arc/tracing"
	"github.com/ordishs/go-utils"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ClientI is the interface for the callbacker transactionHandler
type ClientI interface {
	RegisterCallback(ctx context.Context, callback *callbacker_api.Callback) error
}

type Client struct {
	address string
	logger  utils.Logger
}

func NewClient(l utils.Logger, address string) *Client {
	return &Client{
		address: address,
		logger:  l,
	}
}

func (cb *Client) RegisterCallback(ctx context.Context, callback *callbacker_api.Callback) error {
	conn, err := cb.dialGRPC()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := callbacker_api.NewCallbackerAPIClient(conn)

	_, err = client.RegisterCallback(ctx, callback)
	if err != nil {
		return err
	}

	return nil
}

func (cb *Client) dialGRPC() (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`), // This sets the initial balancing policy.
	}

	return grpc.Dial(cb.address, tracing.AddGRPCDialOptions(opts)...)
}
