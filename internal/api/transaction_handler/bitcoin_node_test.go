package transaction_handler

import (
	"context"
	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewBitcoinNode(t *testing.T) {
	t.Run("new bitcoin node", func(t *testing.T) {
		//Given
		// This is the genesis coinbase transaction that is hardcoded and does not need connection to anything else
		tx1 := "4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b"

		// add a single bitcoin node
		txHandler, err := NewBitcoinNode("localhost", 8332, "user", "mypassword", false)
		require.NoError(t, err)
		var ctx = context.Background()

		//When

		//Then

		err = txHandler.Health(ctx)
		require.NoError(t, err)

		res1, err := txHandler.GetTransaction(ctx, tx1)
		require.NotNil(t, res1)
		require.NoError(t, err)

		res2, err := txHandler.GetTransactions(ctx, []string{tx1, tx1})
		require.NotNil(t, res2)
		require.NoError(t, err)

		res3, err := txHandler.GetTransactionStatuses(ctx, []string{tx1, tx1})
		require.NotNil(t, res3)
		require.NoError(t, err)

		res4, err := txHandler.GetTransactionStatus(ctx, tx1)
		require.NotNil(t, res4)
		require.NoError(t, err)

		res5, err := txHandler.SubmitTransaction(ctx, &sdkTx.Transaction{}, nil)
		require.Nil(t, res5)
		require.Error(t, err)

		res6, err := txHandler.SubmitTransactions(ctx, sdkTx.Transactions{}, nil)
		require.NotNil(t, res6)
		require.NoError(t, err)
	})
}
