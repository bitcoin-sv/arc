package blocktx

import (
	"context"
	"errors"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"google.golang.org/protobuf/types/known/emptypb"
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
	GetCompetingTransactionStatuses(ctx context.Context, hash [][]byte) (bool, error)
	RegisterTransaction(ctx context.Context, hash []byte) error
	RegisterTransactions(ctx context.Context, hashes [][]byte) error
	CurrentBlockHeight(ctx context.Context) (*blocktx_api.CurrentBlockHeightResponse, error)
	VerifyMerkleRoots(ctx context.Context, merkleRootVerificationRequest []MerkleRootVerificationRequest) ([]uint64, error)
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

func (btc *BtxClient) GetCompetingTransactionStatuses(ctx context.Context, hash [][]byte) (bool, error) {
	mined, err := btc.client.GetCompetingTransactionStatuses(ctx,
		&blocktx_api.CompetingTxs{
			CompetingTxs: hash,
		})
	if err != nil {
		return false, err
	}

	return mined.Mined, nil
}

func (btc *BtxClient) RegisterTransactions(ctx context.Context, hashes [][]byte) error {
	var txs []*blocktx_api.Transaction
	for _, hash := range hashes {
		txs = append(txs, &blocktx_api.Transaction{Hash: hash})
	}

	_, err := btc.client.RegisterTransactions(ctx, &blocktx_api.Transactions{Transactions: txs})
	if err != nil {
		return errors.Join(ErrRegisterTransaction, err)
	}

	return nil
}

func (btc *BtxClient) CurrentBlockHeight(ctx context.Context) (*blocktx_api.CurrentBlockHeightResponse, error) {
	return btc.client.CurrentBlockHeight(ctx, &emptypb.Empty{})
}
