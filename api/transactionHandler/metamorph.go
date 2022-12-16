package transactionHandler

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	arc "github.com/TAAL-GmbH/arc/api"
	btcpb "github.com/TAAL-GmbH/arc/blocktx/api"
	metamorph_api2 "github.com/TAAL-GmbH/arc/metamorph/api"
	"github.com/ordishs/go-bitcoin"
	"github.com/ordishs/go-utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Metamorph is the connector to a metamorph server
type Metamorph struct {
	Servers         map[string]metamorph_api2.MetaMorphAPIClient
	locationService MetamorphLocationI
}

// NewMetamorph creates a connection to a list of metamorph servers via gRPC
func NewMetamorph(targets []string, locationService MetamorphLocationI) (metamorph *Metamorph, err error) {
	servers := make(map[string]metamorph_api2.MetaMorphAPIClient)
	for _, target := range targets {
		var server *grpc.ClientConn
		server, err = grpc.Dial(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, err
		}
		servers[target] = metamorph_api2.NewMetaMorphAPIClient(server)
	}

	return &Metamorph{
		Servers:         servers,
		locationService: locationService,
	}, nil
}

// GetTransaction gets a raw transaction from the bitcoin node
func (m *Metamorph) GetTransaction(ctx context.Context, txID string) (rawTx *RawTransaction, err error) {
	var client metamorph_api2.MetaMorphAPIClient
	if client, err = m.getMetamorphClientForTx(ctx, txID); err != nil {
		return nil, err
	}

	var tx *metamorph_api2.TransactionStatus
	if tx, err = client.GetTransactionStatus(ctx, &metamorph_api2.TransactionStatusRequest{
		Txid: txID,
	}); err != nil {
		return nil, err
	}

	fmt.Printf("tx: %v\n", tx)

	return &RawTransaction{
		RawTransaction: bitcoin.RawTransaction{
			TxID: txID,
			// Time:          tx.GetCreatedAt(), // created at should be a time
			Blocktime:   0,
			BlockHash:   "", // TODO add to proto
			BlockHeight: 0,  // TODO add to proto
		},
		Status: tx.Status.String(),
	}, nil
}

// GetTransactionStatus gets the status of a transaction
func (m *Metamorph) GetTransactionStatus(ctx context.Context, txID string) (status *TransactionStatus, err error) {
	var client metamorph_api2.MetaMorphAPIClient
	if client, err = m.getMetamorphClientForTx(ctx, txID); err != nil {
		return nil, err
	}

	var tx *metamorph_api2.TransactionStatus
	tx, err = client.GetTransactionStatus(ctx, &metamorph_api2.TransactionStatusRequest{
		Txid: txID,
	})
	if err != nil {
		return nil, err
	}

	if tx == nil {
		// TODO get the transaction from the bitcoin node rpc
		return nil, fmt.Errorf("transaction not found")
	}

	return &TransactionStatus{
		TxID:        txID,
		Status:      tx.Status.String(),
		BlockHash:   "",
		BlockHeight: 0,
		Timestamp:   time.Now().Unix(),
	}, nil
}

// SubmitTransaction submits a transaction to the bitcoin network and returns the transaction in raw format
func (m *Metamorph) SubmitTransaction(ctx context.Context, tx []byte, _ *arc.TransactionOptions) (*TransactionStatus, error) {
	_, client, err := m.getRandomMetamorphClient()
	if err != nil {
		return nil, err
	}

	// TODO add retry logic if the put fails
	var response *metamorph_api2.TransactionStatus
	response, err = client.PutTransaction(ctx, &metamorph_api2.TransactionRequest{
		RawTx: tx,
	})
	if err != nil {
		return nil, err
	}

	return &TransactionStatus{
		TxID:        response.Txid,
		Status:      response.Status.String(),
		BlockHash:   "", // TODO proto
		BlockHeight: 0,  // TODO proto
		Timestamp:   time.Now().Unix(),
	}, nil
}

func (m *Metamorph) getRandomMetamorphClient() (string, metamorph_api2.MetaMorphAPIClient, error) {
	k := rand.Intn(len(m.Servers))
	for target, server := range m.Servers {
		if k == 0 {
			return target, server, nil
		}
		k--
	}

	return "", nil, fmt.Errorf("no metamorph server could be selected")
}

func (m *Metamorph) getMetamorphClientForTx(ctx context.Context, txID string) (metamorph_api2.MetaMorphAPIClient, error) {
	hash, err := utils.DecodeAndReverseHexString(txID)
	if err != nil {
		return nil, err
	}

	target, err := m.locationService.GetServer(ctx, &btcpb.Transaction{
		Hash: hash,
	})
	if err != nil {
		return nil, err
	}

	if target == "" {
		// TODO what do we do in this case? Reach out to all metamorph servers or reach out to a node?
		return nil, fmt.Errorf("could not find transaction server")
	}

	return m.Servers[target], nil
}
