package broadcaster

import (
	"context"

	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
)

type ArcClient interface {
	BroadcastTransaction(ctx context.Context, tx *sdkTx.Transaction, waitForStatus metamorph_api.Status, callbackURL string) (*metamorph_api.TransactionStatus, error)
	BroadcastTransactions(ctx context.Context, txs sdkTx.Transactions, waitForStatus metamorph_api.Status, callbackURL string, callbackToken string, fullStatusUpdates bool, skipFeeValidation bool) ([]*metamorph_api.TransactionStatus, error)
	GetTransactionStatus(ctx context.Context, txID string) (*metamorph_api.TransactionStatus, error)
}
