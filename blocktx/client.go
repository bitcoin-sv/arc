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
	Shutdown()
}

const (
	logLevelDefault      = slog.LevelInfo
	retryIntervalDefault = 10 * time.Second
)

type Client struct {
	logger           *slog.Logger
	client           blocktx_api.BlockTxAPIClient
	retryInterval    time.Duration
	getBlocksTicker  *time.Ticker
	shutdownComplete chan struct{}
	shutdown         chan struct{}
}

func WithLogger(logger *slog.Logger) func(*Client) {
	return func(p *Client) {
		p.logger = logger.With(slog.String("service", "btx"))
	}
}

func WithRetryInterval(d time.Duration) func(*Client) {
	return func(p *Client) {
		p.retryInterval = d
	}
}

type Option func(f *Client)

func NewClient(client blocktx_api.BlockTxAPIClient, opts ...Option) ClientI {
	btc := &Client{
		logger:           slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevelDefault})).With(slog.String("service", "btx")),
		client:           client,
		retryInterval:    retryIntervalDefault,
		shutdown:         make(chan struct{}, 1),
		shutdownComplete: make(chan struct{}, 1),
	}

	for _, opt := range opts {
		opt(btc)
	}

	return btc
}

func (btc *Client) Start(minedBlockChan chan *blocktx_api.Block) {
	btc.getBlocksTicker = time.NewTicker(btc.retryInterval)
	go func() {
		defer func() {
			btc.shutdownComplete <- struct{}{}
		}()

		for {
			select {
			case <-btc.getBlocksTicker.C:
				stream, err := btc.client.GetBlockNotificationStream(context.Background(), &blocktx_api.Height{})
				if err != nil {
					btc.logger.Error("failed to get block notification stream", slog.String("err", err.Error()))
					continue
				}

				btc.logger.Info("Connected to block-tx server")

				var block *blocktx_api.Block
				for {
					select {
					case <-btc.shutdown:
						return
					default:
						block, err = stream.Recv()
						if err != nil {
							btc.logger.Error("Failed to receive block", slog.String("err", err.Error()))
							break
						}
						btc.logger.Info("Block", slog.String("hash", utils.ReverseAndHexEncodeSlice(block.Hash)))
						utils.SafeSend(minedBlockChan, block)
					}
				}
			case <-btc.shutdown:
				return
			}
		}
	}()
}

func (btc *Client) Shutdown() {
	btc.getBlocksTicker.Stop()
	btc.shutdown <- struct{}{}

	<-btc.shutdownComplete
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

func DialGRPC(address string) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpc_prometheus.UnaryClientInterceptor),
		grpc.WithChainStreamInterceptor(grpc_prometheus.StreamClientInterceptor),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`), // This sets the initial balancing policy.
	}

	return grpc.Dial(address, tracing.AddGRPCDialOptions(opts)...)
}
