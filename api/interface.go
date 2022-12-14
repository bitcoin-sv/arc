package api

// HandlerInterface is an interface for implementations of the ARC backends
// this is an extension of the generated interface, to allow additional methods
type HandlerInterface interface {
	ServerInterface
}

// TransactionOptions options passed from header when creating transactions
type TransactionOptions struct {
	ClientID      string `json:"client_id"`
	CallbackURL   string `json:"callback_url,omitempty"`
	CallbackToken string `json:"callback_token,omitempty"`
	MerkleProof   bool   `json:"merkle_proof,omitempty"`
}
