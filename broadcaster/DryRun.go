package broadcaster

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/libsv/go-bt/v2"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type DryRunClient struct {
}

func NewDryRunClient() ClientI {
	return &DryRunClient{}
}

func (d DryRunClient) Health(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*metamorph_api.HealthResponse, error) {
	return &metamorph_api.HealthResponse{
		Ok:        false,
		Details:   "Dry Run client",
		Timestamp: timestamppb.New(time.Now()),
		Workers:   0,
		Uptime:    0,
		Queued:    0,
		Processed: 0,
		Waiting:   0,
		Average:   0,
		MapSize:   0,
	}, nil
}

func (d DryRunClient) PutTransaction(ctx context.Context, tx *bt.Tx) (*metamorph_api.TransactionStatus, error) {
	fmt.Printf("%s\n\n", hex.EncodeToString(tx.Bytes()))
	return &metamorph_api.TransactionStatus{}, nil
}

func (d DryRunClient) GetTransactionStatus(ctx context.Context, txID string) (*metamorph_api.TransactionStatus, error) {
	//TODO implement me
	panic("implement me")
}
