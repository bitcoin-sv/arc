package blocktx

import (
	"context"
	"time"

	"github.com/TAAL-GmbH/arc/store"

	btcpb "github.com/TAAL-GmbH/arc/blocktx_api"
	pb "github.com/TAAL-GmbH/arc/metamorph_api"

	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type BlockTxClient struct {
	store  store.Store
	logger utils.Logger
}

func New(s store.Store, l utils.Logger) *BlockTxClient {
	return &BlockTxClient{
		store:  s,
		logger: l,
	}
}

func (btc *BlockTxClient) Start() {
	for {
		address, _ := gocore.Config().Get("blockTxAddress", "localhost:8001")

		conn, _ := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
		defer conn.Close()

		client := btcpb.NewBlockTxAPIClient(conn)

		stream, err := client.GetMinedBlockTransactions(context.Background(), &btcpb.HeightAndSource{})
		if err != nil {
			panic(err)
		}

		btc.logger.Infof("Connected to block-tx server at %s", address)

		ctx := context.Background()

		for {
			mt, err := stream.Recv()
			if err != nil {
				break
			}

			btc.logger.Infof("Block %x\n", mt.Blockhash)
			for _, tx := range mt.Txs {
				if err := btc.store.UpdateStatus(ctx, tx, pb.Status_MINED, ""); err != nil {
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
