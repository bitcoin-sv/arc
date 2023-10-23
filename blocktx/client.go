package blocktx

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/tracing"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/ordishs/go-utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ClientI is the interface for the block-tx transactionHandler
type ClientI interface {
	Start(minedBlockChan chan *blocktx_api.Block)
	LocateTransaction(ctx context.Context, transaction *blocktx_api.Transaction) (string, error)
	GetTransactionMerklePath(ctx context.Context, transaction *blocktx_api.Transaction) (string, error)
	RegisterTransaction(ctx context.Context, transaction *blocktx_api.TransactionAndSource) (*blocktx_api.RegisterTransactionResponse, error)
	GetTransactionBlocks(ctx context.Context, transaction *blocktx_api.Transactions) (*blocktx_api.TransactionBlocks, error)
	GetTransactionBlock(ctx context.Context, transaction *blocktx_api.Transaction) (*blocktx_api.RegisterTransactionResponse, error)
	GetBlock(ctx context.Context, blockHash *chainhash.Hash) (*blocktx_api.Block, error)
	GetLastProcessedBlock(ctx context.Context) (*blocktx_api.Block, error)
	GetMinedTransactionsForBlock(ctx context.Context, blockAndSource *blocktx_api.BlockAndSource) (*blocktx_api.MinedTransactions, error)
}

const (
	logLevelDefault = slog.LevelInfo
)

type Client struct {
	address string
	logger  *slog.Logger
	conn    *grpc.ClientConn
	client  blocktx_api.BlockTxAPIClient
}

func WithLogger(logger *slog.Logger) func(*Client) {
	return func(p *Client) {
		p.logger = logger.With(slog.String("service", "btx"))
	}
}

type Option func(f *Client)

func NewClient(address string, opts ...Option) ClientI {
	btc := &Client{
		address: address,
		logger:  slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevelDefault})).With(slog.String("service", "btx")),
	}

	for _, opt := range opts {
		opt(btc)
	}

	conn, err := btc.dialGRPC()
	if err != nil {
		btc.logger.Error("Failed to connect to block-tx server", slog.String("address", btc.address), slog.String("err", err.Error()))
	}
	btc.logger.Info("Connected to block-tx server", slog.String("address", btc.address))

	btc.conn = conn
	btc.client = blocktx_api.NewBlockTxAPIClient(conn)

	return btc
}

func (btc *Client) Start(minedBlockChan chan *blocktx_api.Block) {
	for {
		stream, err := btc.client.GetBlockNotificationStream(context.Background(), &blocktx_api.Height{})
		if err != nil {
			btc.logger.Error("Error getting block notification stream", slog.String("err", err.Error()))
		} else {
			btc.logger.Info("Connected to block-tx server", slog.String("address", btc.address))

			var block *blocktx_api.Block
			for {
				block, err = stream.Recv()
				if err != nil {
					break
				}

				btc.logger.Info("Block", slog.String("hash", utils.ReverseAndHexEncodeSlice(block.Hash)))
				utils.SafeSend(minedBlockChan, block)
			}
		}

		time.Sleep(10 * time.Second)
	}
}

func (btc *Client) LocateTransaction(ctx context.Context, transaction *blocktx_api.Transaction) (string, error) {
	location, err := btc.client.LocateTransaction(ctx, transaction)
	if err != nil {
		return "", ErrTransactionNotFound
	}

	return location.Source, nil
}

func (btc *Client) GetTransactionMerklePath(ctx context.Context, hash *blocktx_api.Transaction) (string, error) {
	merklePath, err := btc.client.GetTransactionMerklePath(ctx, hash)
	if err != nil {
		return "", err
	}

	return merklePath.MerklePath, nil
}

func (btc *Client) RegisterTransaction(ctx context.Context, transaction *blocktx_api.TransactionAndSource) (*blocktx_api.RegisterTransactionResponse, error) {
	return btc.client.RegisterTransaction(ctx, transaction)
}

func (btc *Client) GetTransactionBlock(ctx context.Context, transaction *blocktx_api.Transaction) (*blocktx_api.RegisterTransactionResponse, error) {
	block, err := btc.client.GetTransactionBlock(ctx, transaction)
	if err != nil {
		return nil, err
	}

	return &blocktx_api.RegisterTransactionResponse{
		BlockHash:   block.Hash,
		BlockHeight: block.Height,
	}, nil
}

func (btc *Client) GetTransactionBlocks(ctx context.Context, transaction *blocktx_api.Transactions) (*blocktx_api.TransactionBlocks, error) {
	transactionBlocks, err := btc.client.GetTransactionBlocks(ctx, transaction)
	if err != nil {
		return nil, err
	}

	return transactionBlocks, nil
}

func (btc *Client) GetBlock(ctx context.Context, blockHash *chainhash.Hash) (*blocktx_api.Block, error) {
	block, err := btc.client.GetBlock(ctx, &blocktx_api.Hash{
		Hash: blockHash[:],
	})
	if err != nil {
		return nil, err
	}

	return block, nil
}

func (btc *Client) GetLastProcessedBlock(ctx context.Context) (*blocktx_api.Block, error) {
	block, err := btc.client.GetLastProcessedBlock(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, err
	}

	return block, nil
}

func (btc *Client) GetMinedTransactionsForBlock(ctx context.Context, blockAndSource *blocktx_api.BlockAndSource) (*blocktx_api.MinedTransactions, error) {
	mt, err := btc.client.GetMinedTransactionsForBlock(ctx, blockAndSource)
	if err != nil {
		return nil, err
	}

	return mt, nil
}

func (btc *Client) dialGRPC() (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpc_prometheus.UnaryClientInterceptor),
		grpc.WithChainStreamInterceptor(grpc_prometheus.StreamClientInterceptor),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`), // This sets the initial balancing policy.
	}

	return grpc.Dial(btc.address, tracing.AddGRPCDialOptions(opts)...)
}
