package transactionHandler

import (
	"context"
	"fmt"
	"sync"
	"time"

	arc "github.com/TAAL-GmbH/arc/api"
	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"
	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/ordishs/go-bitcoin"
	"github.com/ordishs/go-utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Metamorph is the connector to a metamorph server
type Metamorph struct {
	mu              sync.RWMutex
	Client          metamorph_api.MetaMorphAPIClient
	ClientCache     map[string]metamorph_api.MetaMorphAPIClient
	locationService MetamorphLocationI
}

// NewMetamorph creates a connection to a list of metamorph servers via gRPC
func NewMetamorph(targets string, locationService MetamorphLocationI) (metamorph *Metamorph, err error) {
	conn, err := grpc.Dial(targets,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`), // This sets the initial balancing policy.
	)
	if err != nil {
		return nil, err
	}

	return &Metamorph{
		Client:          metamorph_api.NewMetaMorphAPIClient(conn),
		locationService: locationService,
	}, nil
}

// GetTransaction gets a raw transaction from the bitcoin node
func (m *Metamorph) GetTransaction(ctx context.Context, txID string) (rawTx *RawTransaction, err error) {
	var client metamorph_api.MetaMorphAPIClient
	if client, err = m.getMetamorphClientForTx(ctx, txID); err != nil {
		return nil, err
	}

	var tx *metamorph_api.TransactionStatus
	if tx, err = client.GetTransactionStatus(ctx, &metamorph_api.TransactionStatusRequest{
		Txid: txID,
	}); err != nil {
		return nil, err
	}

	// fmt.Printf("tx: %v\n", tx)

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
	var client metamorph_api.MetaMorphAPIClient
	if client, err = m.getMetamorphClientForTx(ctx, txID); err != nil {
		return nil, err
	}

	var tx *metamorph_api.TransactionStatus
	tx, err = client.GetTransactionStatus(ctx, &metamorph_api.TransactionStatusRequest{
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
	// TODO add retry logic if the put fails
	response, err := m.Client.PutTransaction(ctx, &metamorph_api.TransactionRequest{
		RawTx: tx,
	})
	if err != nil {
		return nil, err
	}

	return &TransactionStatus{
		TxID:        response.Txid,
		Status:      response.GetStatus().String(),
		BlockHash:   "", // TODO proto
		BlockHeight: 0,  // TODO proto
		Timestamp:   time.Now().Unix(),
	}, nil
}

func (m *Metamorph) getMetamorphClientForTx(ctx context.Context, txID string) (metamorph_api.MetaMorphAPIClient, error) {
	hash, err := utils.DecodeAndReverseHexString(txID)
	if err != nil {
		return nil, err
	}

	target, err := m.locationService.GetServer(ctx, &blocktx_api.Transaction{
		Hash: hash,
	})
	if err != nil {
		return nil, err
	}

	if target == "" {
		// TODO what do we do in this case? Reach out to all metamorph servers or reach out to a node?
		return nil, fmt.Errorf("could not find transaction server")
	}

	m.mu.RLock()
	client, found := m.ClientCache[target]
	m.mu.RUnlock()

	if !found {
		conn, err := grpc.Dial(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, err
		}
		client = metamorph_api.NewMetaMorphAPIClient(conn)

		m.mu.Lock()
		m.ClientCache[target] = client
		m.mu.Unlock()
	}

	return client, nil
}
