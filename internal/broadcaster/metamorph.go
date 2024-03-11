package broadcaster

import (
	"context"

	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/tracing"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/libsv/go-bt/v2"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type MetamorphBroadcaster struct {
	address string
	client  metamorph_api.MetaMorphAPIClient
}

func NewMetamorphBroadcaster(address string) (*MetamorphBroadcaster, error) {
	addresses := viper.GetString("metamorph.dialAddr")

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpc_prometheus.UnaryClientInterceptor),
		grpc.WithChainStreamInterceptor(grpc_prometheus.StreamClientInterceptor),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`), // This sets the initial balancing policy.grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`), // This sets the initial balancing policy.
	}

	cc, err := grpc.DialContext(context.Background(), addresses, tracing.AddGRPCDialOptions(opts)...)
	if err != nil {
		return nil, err
	}

	client := metamorph_api.NewMetaMorphAPIClient(cc)

	return &MetamorphBroadcaster{
		address: address,
		client:  client,
	}, nil
}

func (m *MetamorphBroadcaster) BroadcastTransactions(ctx context.Context, txs []*bt.Tx, waitFor metamorph_api.Status, callbackURL string, callbackToken string) ([]*metamorph_api.TransactionStatus, error) {
	txStatuses := make([]*metamorph_api.TransactionStatus, len(txs))
	for idx, tx := range txs {

		req := &metamorph_api.TransactionRequest{
			RawTx:         tx.Bytes(),
			WaitForStatus: waitFor,
		}

		if callbackURL != "" {
			req.CallbackUrl = callbackURL
		}

		txStatus, err := m.client.PutTransaction(ctx, req)
		if err != nil {
			// return nil, err
			// we should not return here, but continue with the next tx and mark this one as failed
			// we do that by setting the version to 0, which should then be read by the consolidator
			tx.Version = 0
		}
		txStatuses[idx] = txStatus
	}

	return txStatuses, nil
}

func (m *MetamorphBroadcaster) BroadcastTransaction(ctx context.Context, tx *bt.Tx, waitFor metamorph_api.Status, callbackURL string) (*metamorph_api.TransactionStatus, error) {
	req := &metamorph_api.TransactionRequest{
		RawTx:         tx.Bytes(),
		WaitForStatus: waitFor,
	}

	if callbackURL != "" {
		req.CallbackUrl = callbackURL
	}

	return m.client.PutTransaction(ctx, req)
}

func (m *MetamorphBroadcaster) GetTransactionStatus(ctx context.Context, txID string) (*metamorph_api.TransactionStatus, error) {
	return nil, nil
}
