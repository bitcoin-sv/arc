package validator

import (
	"context"

	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
	"go.opentelemetry.io/otel/attribute"
)

type FeeValidation byte

const (
	NoneFeeValidation FeeValidation = iota
	StandardFeeValidation
	CumulativeFeeValidation
)

const DustLimit = 1

type ScriptValidation byte

const (
	NoneScriptValidation ScriptValidation = iota
	StandardScriptValidation
)

type DefaultValidator interface {
	ValidateTransaction(ctx context.Context, tx *sdkTx.Transaction, feeValidation FeeValidation, scriptValidation ScriptValidation, tracingEnabled bool, tracingAttributes ...attribute.KeyValue) error
}

type BeefValidator interface {
	ValidateTransaction(ctx context.Context, beefTx *sdkTx.Beef, txID string, feeValidation FeeValidation, scriptValidation ScriptValidation, tracingEnabled bool, tracingAttributes ...attribute.KeyValue) error
}
