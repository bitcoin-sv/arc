package blocktx

import (
	"context"
	"time"

	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"
	"github.com/ordishs/go-utils"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ClientI is the interface for the block-tx transactionHandler
type ClientI interface {
	Start(minedBlockChan chan *blocktx_api.Block)
	LocateTransaction(ctx context.Context, transaction *blocktx_api.Transaction) (string, error)
	RegisterTransaction(ctx context.Context, transaction *blocktx_api.TransactionAndSource) error
	GetMinedTransactionsForBlock(ctx context.Context, blockAndSource *blocktx_api.BlockAndSource) (*blocktx_api.MinedTransactions, error)
}

type Client struct {
	address string
	logger  utils.Logger
}

func NewClient(l utils.Logger, address string) ClientI {
	return &Client{
		address: address,
		logger:  l,
	}
}

func (btc *Client) Start(minedBlockChan chan *blocktx_api.Block) {
	for {
		conn, _ := grpc.Dial(btc.address, grpc.WithTransportCredentials(insecure.NewCredentials()))
		defer conn.Close()

		client := blocktx_api.NewBlockTxAPIClient(conn)

		stream, err := client.GetBlockNotificationStream(context.Background(), &blocktx_api.Height{})
		if err != nil {
			panic(err)
		}

		btc.logger.Infof("Connected to block-tx server at %s", btc.address)

		for {
			block, err := stream.Recv()
			if err != nil {
				break
			}

			btc.logger.Infof("Block %s\n", utils.HexEncodeAndReverseBytes(block.Hash))
			utils.SafeSend(minedBlockChan, block)
		}

		btc.logger.Warnf("could not get message from block-tx stream: %v", err)
		conn.Close()
		btc.logger.Warnf("Retrying in 10 seconds")
		time.Sleep(10 * time.Second)
	}
}

func (btc *Client) LocateTransaction(ctx context.Context, transaction *blocktx_api.Transaction) (string, error) {
	conn, err := grpc.Dial(btc.address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return "", err
	}

	defer conn.Close()

	client := blocktx_api.NewBlockTxAPIClient(conn)

	location, err := client.LocateTransaction(ctx, transaction)
	if err != nil {
		return "", err
	}

	return location.Source, nil
}

func (btc *Client) RegisterTransaction(ctx context.Context, transaction *blocktx_api.TransactionAndSource) error {
	conn, err := grpc.Dial(btc.address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}

	defer conn.Close()

	client := blocktx_api.NewBlockTxAPIClient(conn)

	if _, err := client.RegisterTransaction(ctx, transaction); err != nil {
		return err
	}

	return nil
}

func (btc *Client) GetMinedTransactionsForBlock(ctx context.Context, blockAndSource *blocktx_api.BlockAndSource) (*blocktx_api.MinedTransactions, error) {
	conn, err := grpc.Dial(btc.address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	defer conn.Close()

	client := blocktx_api.NewBlockTxAPIClient(conn)

	mt, err := client.GetMinedTransactionsForBlock(ctx, blockAndSource)
	if err != nil {
		return nil, err
	}

	return mt, nil
}
