package broadcaster_test

import (
	"container/list"
	"context"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"log/slog"
	"os"

	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/internal/testdata"
	"github.com/bitcoin-sv/arc/pkg/keyset"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/bscript"
	"github.com/stretchr/testify/require"
)

const (
	batchSizeDefault = 20
	maxInputsDefault = 100
)

func TestBroadcasterWithPredefinedUTXOs(t *testing.T) {
	// Set up a logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create a KeySet
	ks1, err := keyset.New()
	require.NoError(t, err)

	// Define UTXOs
	utxo1 := &bt.UTXO{
		TxID:          testdata.TX1Hash[:],
		Vout:          0,
		LockingScript: ks1.Script,
		Satoshis:      1000,
	}
	utxo2 := &bt.UTXO{
		TxID:          testdata.TX2Hash[:],
		Vout:          0,
		LockingScript: ks1.Script,
		Satoshis:      2000,
	}
	utxo3 := &bt.UTXO{
		TxID:          testdata.TX3Hash[:],
		Vout:          0,
		LockingScript: ks1.Script,
		Satoshis:      3000,
	}

	// Mock UTXO Client
	utxoClient := &MockUtxoClient{
		utxos: []*bt.UTXO{utxo1, utxo2, utxo3},
	}

	// Initialize the Broadcaster
	b, err := broadcaster.NewBroadcaster(logger, nil, utxoClient, false)
	require.NoError(t, err)

	// Use reflection to access the private fields (for testing purposes only)
	batchSize := reflect.ValueOf(b).FieldByName("batchSize").Int()
	maxInputs := reflect.ValueOf(b).FieldByName("maxInputs").Int()

	// Ensure the broadcaster is using the default batch size and max inputs
	require.Equal(t, int64(batchSizeDefault), batchSize)
	require.Equal(t, int64(maxInputsDefault), maxInputs)

	//To do include test for other functions

}

type MockUtxoClient struct {
	utxos []*bt.UTXO
}

func (m *MockUtxoClient) GetUTXOs(ctx context.Context, mainnet bool, lockingScript *bscript.Script, address string) ([]*bt.UTXO, error) {
	return m.utxos, nil
}

func (m *MockUtxoClient) GetUTXOsWithRetries(ctx context.Context, mainnet bool, lockingScript *bscript.Script, address string, constantBackoff time.Duration, retries uint64) ([]*bt.UTXO, error) {
	return m.utxos, nil
}

func (m *MockUtxoClient) GetUTXOsList(ctx context.Context, mainnet bool, lockingScript *bscript.Script, address string) (*list.List, error) {
	list := list.New()
	for _, utxo := range m.utxos {
		list.PushBack(utxo)
	}
	return list, nil
}

func (m *MockUtxoClient) GetUTXOsListWithRetries(ctx context.Context, mainnet bool, lockingScript *bscript.Script, address string, constantBackoff time.Duration, retries uint64) (*list.List, error) {
	list := list.New()
	for _, utxo := range m.utxos {
		list.PushBack(utxo)
	}
	return list, nil
}

func (m *MockUtxoClient) GetBalance(ctx context.Context, mainnet bool, address string) (int64, int64, error) {
	var balance int64
	for _, utxo := range m.utxos {
		if utxo.Satoshis > math.MaxInt64 {
			return 0, 0, fmt.Errorf("Satoshis value is too large to convert to int64: %d", utxo.Satoshis)
		}
		balance += int64(utxo.Satoshis)
	}
	return balance, 0, nil
}

func (m *MockUtxoClient) GetBalanceWithRetries(ctx context.Context, mainnet bool, address string, constantBackoff time.Duration, retries uint64) (int64, int64, error) {
	var balance int64
	for _, utxo := range m.utxos {
		if utxo.Satoshis > math.MaxInt64 {
			return 0, 0, fmt.Errorf("Satoshis value is too large to convert to int64: %d", utxo.Satoshis)
		}
		balance += int64(utxo.Satoshis)
	}
	return balance, 0, nil
}

func (m *MockUtxoClient) TopUp(ctx context.Context, mainnet bool, address string) error {
	return nil
}
