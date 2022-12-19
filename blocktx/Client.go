package blocktx

import (
	"context"
	"time"

	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"
	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/ordishs/go-utils"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ClientI is the interface for the block-tx transactionHandler
type ClientI interface {
	Start(s store.Store)
	LocateTransaction(ctx context.Context, transaction *blocktx_api.Transaction) (string, error)
	RegisterTransaction(ctx context.Context, transaction *blocktx_api.Transaction) error
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

func (btc *Client) Start(s store.Store) {
	for {
		conn, _ := grpc.Dial(btc.address, grpc.WithTransportCredentials(insecure.NewCredentials()))
		defer conn.Close()

		client := blocktx_api.NewBlockTxAPIClient(conn)

		stream, err := client.GetMinedBlockTransactions(context.Background(), &blocktx_api.HeightAndSource{})
		if err != nil {
			panic(err)
		}

		btc.logger.Infof("Connected to block-tx server at %s", btc.address)

		ctx := context.Background()

		for {
			mt, err := stream.Recv()
			if err != nil {
				break
			}

			btc.logger.Infof("Block %x\n", utils.ReverseSlice(mt.Block.Hash))
			for _, tx := range mt.Txs {
				if err := s.UpdateMined(ctx, tx.Hash, mt.Block.Hash, int32(mt.Block.Height)); err != nil {
					btc.logger.Errorf("Could not update status of %x to %s: %v", utils.ReverseSlice(tx.Hash), metamorph_api.Status_MINED, err)
				}
			}
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

func (btc *Client) RegisterTransaction(ctx context.Context, transaction *blocktx_api.Transaction) error {
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
