package blocktx

import (
	"context"
	"errors"

	"google.golang.org/grpc"

	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/grpc_utils"
)

// check if BtxClient implements all necessary interfaces
var _ MerkleRootsVerifier = &BtxClient{}
var _ Client = &BtxClient{}

// MerkleRootsVerifier verifies the merkle roots existence in blocktx db and returns unverified block heights.
type MerkleRootsVerifier interface {
	// VerifyMerkleRoots verifies the merkle roots existence in blocktx db and returns unverified block heights.
	VerifyMerkleRoots(ctx context.Context, merkleRootVerificationRequest []MerkleRootVerificationRequest) ([]uint64, error)
}

var (
	ErrRegisterTransaction = errors.New("failed to register transaction")
)

type Client interface {
	RegisterTransaction(ctx context.Context, hash []byte) error
}

type MerkleRootVerificationRequest struct {
	MerkleRoot  string
	BlockHeight uint64
}

type BtxClient struct {
	client blocktx_api.BlockTxAPIClient
}

func NewClient(client blocktx_api.BlockTxAPIClient) *BtxClient {
	btc := &BtxClient{
		client: client,
	}

	return btc
}

func (btc *BtxClient) VerifyMerkleRoots(ctx context.Context, merkleRootVerificationRequest []MerkleRootVerificationRequest) ([]uint64, error) {
	merkleRoots := make([]*blocktx_api.MerkleRootVerificationRequest, 0)

	for _, mr := range merkleRootVerificationRequest {
		merkleRoots = append(merkleRoots, &blocktx_api.MerkleRootVerificationRequest{MerkleRoot: mr.MerkleRoot, BlockHeight: mr.BlockHeight})
	}

	resp, err := btc.client.VerifyMerkleRoots(ctx, &blocktx_api.MerkleRootsVerificationRequest{MerkleRoots: merkleRoots})
	if err != nil {
		return nil, err
	}

	return resp.UnverifiedBlockHeights, nil
}

func (btc *BtxClient) RegisterTransaction(ctx context.Context, hash []byte) error {
	_, err := btc.client.RegisterTransaction(ctx, &blocktx_api.Transaction{Hash: hash})
	if err != nil {
		return errors.Join(ErrRegisterTransaction, err)
	}

	return nil
}

func DialGRPC(address string, prometheusEndpoint string, grpcMessageSize int, tracingConfig *config.TracingConfig) (*grpc.ClientConn, error) {
	dialOpts, err := grpc_utils.GetGRPCClientOpts(prometheusEndpoint, grpcMessageSize, tracingConfig)
	if err != nil {
		return nil, err
	}

	return grpc.NewClient(address, dialOpts...)
}
