package api

import (
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
)

// ContextKey type.
type ContextKey int

const (
	ContextSizings ContextKey = iota
)

// TransactionOptions options passed from header when creating transactions.
type TransactionOptions struct {
	ClientID             string               `json:"client_id"`
	CallbackURL          string               `json:"callback_url,omitempty"`
	CallbackToken        string               `json:"callback_token,omitempty"`
	SkipFeeValidation    bool                 `json:"X-SkipFeeValidation,omitempty"`
	SkipScriptValidation bool                 `json:"X-SkipScriptValidation,omitempty"`
	SkipTxValidation     bool                 `json:"X-SkipTxValidation,omitempty"`
	MerkleProof          bool                 `json:"merkle_proof,omitempty"`
	WaitForStatus        metamorph_api.Status `json:"wait_for_status,omitempty"`
	FullStatusUpdates    bool                 `json:"full_status_updates,omitempty"`
	MaxTimeout           int                  `json:"max_timeout,omitempty"`
}
