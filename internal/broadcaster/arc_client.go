package broadcaster

import (
	"context"
	"github.com/bitcoin-sv/go-sdk/transaction"

	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
)

type ArcClient interface {
	BroadcastTransaction(ctx context.Context, tx *transaction.Transaction, waitForStatus metamorph_api.Status, callbackURL string) (*metamorph_api.TransactionStatus, error)
	BroadcastTransactions(ctx context.Context, txs transaction.Transactions, waitForStatus metamorph_api.Status, callbackURL string, callbackToken string, fullStatusUpdates bool, skipFeeValidation bool) ([]*metamorph_api.TransactionStatus, error)
	GetTransactionStatus(ctx context.Context, txID string) (*metamorph_api.TransactionStatus, error)
}
