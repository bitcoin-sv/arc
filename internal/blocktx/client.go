package blocktx

import (
	"context"
	"errors"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
)

var ErrRegisterTransaction = errors.New("failed to register transaction")

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

func (btc *BtxClient) AnyTransactionsMined(ctx context.Context, hash [][]byte) ([]*blocktx_api.IsMined, error) {
	txs := &blocktx_api.Transactions{}
	for _, v := range hash {
		txs.Transactions = append(txs.Transactions, &blocktx_api.Transaction{Hash: v})
	}
	mined, err := btc.client.AnyTransactionsMined(ctx, txs)
	if err != nil {
		return nil, err
	}

	return mined.Transactions, nil
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

func (btc *BtxClient) LatestBlocks(ctx context.Context, blocks uint64) (*blocktx_api.LatestBlocksResponse, error) {
	return btc.client.LatestBlocks(ctx, &blocktx_api.NumOfLatestBlocks{Blocks: blocks})
}
