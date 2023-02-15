package api

import (
	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/golang-jwt/jwt"
)

// HandlerInterface is an interface for implementations of the ARC backends
// this is an extension of the generated interface, to allow additional methods
type HandlerInterface interface {
	ServerInterface
}

// TransactionOptions options passed from header when creating transactions
type TransactionOptions struct {
	ClientID      string               `json:"client_id"`
	CallbackURL   string               `json:"callback_url,omitempty"`
	CallbackToken string               `json:"callback_token,omitempty"`
	MerkleProof   bool                 `json:"merkle_proof,omitempty"`
	WaitForStatus metamorph_api.Status `json:"wait_for_status,omitempty"`
}

type JWTCustomClaims struct {
	ClientID string `json:"client_id"`
	Name     string `json:"name"`
	Admin    bool   `json:"admin"`
	jwt.StandardClaims
}

type User struct {
	ClientID string `json:"client_id"`
	Name     string `json:"name"`
	Admin    bool   `json:"admin"`
}

type NodePolicy struct {
	ExcessiveBlockSize              int     `json:"excessiveblocksize"`
	BlockMaxSize                    int     `json:"blockmaxsize"`
	MaxTxSizePolicy                 int     `json:"maxtxsizepolicy"`
	MaxOrphanTxSize                 int     `json:"maxorphantxsize"`
	DataCarrierSize                 int64   `json:"datacarriersize"`
	MaxScriptSizePolicy             int     `json:"maxscriptsizepolicy"`
	MaxOpsPerScriptPolicy           int64   `json:"maxopsperscriptpolicy"`
	MaxScriptNumLengthPolicy        int     `json:"maxscriptnumlengthpolicy"`
	MaxPubKeysPerMultisigPolicy     int64   `json:"maxpubkeyspermultisigpolicy"`
	MaxTxSigopsCountsPolicy         int64   `json:"maxtxsigopscountspolicy"`
	MaxStackMemoryUsagePolicy       int     `json:"maxstackmemoryusagepolicy"`
	MaxStackMemoryUsageConsensus    int     `json:"maxstackmemoryusageconsensus"`
	LimitAncestorCount              int     `json:"limitancestorcount"`
	LimitCPFPGroupMembersCount      int     `json:"limitcpfpgroupmemberscount"`
	MaxMempool                      int     `json:"maxmempool"`
	MaxMempoolSizedisk              int     `json:"maxmempoolsizedisk"`
	MempoolMaxPercentCPFP           int     `json:"mempoolmaxpercentcpfp"`
	AcceptNonStdOutputs             bool    `json:"acceptnonstdoutputs"`
	DataCarrier                     bool    `json:"datacarrier"`
	MinMiningTxFee                  float64 `json:"minminingtxfee"`
	MaxStdTxValidationDuration      int     `json:"maxstdtxvalidationduration"`
	MaxNonStdTxValidationDuration   int     `json:"maxnonstdtxvalidationduration"`
	MaxTxChainValidationBudget      int     `json:"maxtxchainvalidationbudget"`
	ValidationClockCpu              bool    `json:"validationclockcpu"`
	MinConsolidationFactor          int     `json:"minconsolidationfactor"`
	MaxConsolidationInputScriptSize int     `json:"maxconsolidationinputscriptsize"`
	MinConfConsolidationInput       int     `json:"minconfconsolidationinput"`
	MinConsolidationInputMaturity   int     `json:"minconsolidationinputmaturity"`
	AcceptNonStdConsolidationInput  bool    `json:"acceptnonstdconsolidationinput"`
}
