package blocktx

import (
	"context"
	"time"

	btcpb "github.com/TAAL-GmbH/arc/blocktx_api"
	pb "github.com/TAAL-GmbH/arc/metamorph/api"
	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/ordishs/go-utils"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ClientI is the interface for the block-tx transactionHandler
type ClientI interface {
	Start(s store.Store)
	GetTx(ctx interface{}, txID string) (string, error)
	SetTx(ctx context.Context, txID string, server string) error
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

		client := btcpb.NewBlockTxAPIClient(conn)

		stream, err := client.GetMinedBlockTransactions(context.Background(), &btcpb.Height{})
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

			btc.logger.Infof("Block %x\n", mt.Blockhash)
			for _, tx := range mt.Txs {
				if err := s.UpdateStatus(ctx, tx, pb.Status_MINED, ""); err != nil {
					btc.logger.Errorf("Could not update status of %x to %s: %v", utils.ReverseSlice(tx), pb.Status_MINED, err)
				}
			}
		}

		btc.logger.Warnf("could not get message from block-tx stream: %v", err)
		conn.Close()
		btc.logger.Warnf("Retrying in 10 seconds")
		time.Sleep(10 * time.Second)
	}
}

func (btc *Client) GetTx(ctx interface{}, txID string) (string, error) {
	//TODO implement me
	panic("implement me")
}

func (btc *Client) SetTx(ctx context.Context, txID string, server string) error {
	//TODO implement me
	panic("implement me")
}
