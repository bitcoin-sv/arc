package validatoradapter

import (
	"context"
	"errors"

	"github.com/bitcoin-sv/arc/internal/beef"
	"github.com/bitcoin-sv/arc/internal/validator"
	"github.com/libsv/go-bt/v2"
	"github.com/ordishs/go-bitcoin"

	beefValidator "github.com/bitcoin-sv/arc/internal/validator/beef"
	defaultValidator "github.com/bitcoin-sv/arc/internal/validator/default"
)

type ValidatorAdapter interface {
	ValidateTransaction(ctx context.Context, tx any, feeValOpt validator.FeeValidation, scriptValOpt validator.ScriptValidation) (*bt.Tx, error)
}

func GetValidator(nodePolicy *bitcoin.Settings, format validator.HexFormat) ValidatorAdapter {
	switch format {
	case validator.Ef:
		fallthrough
	case validator.Raw:
		return &defaultValidatorAdapter{v: defaultValidator.New(nodePolicy)}

	case validator.Beef:
		return &beefValidatorAdapter{v: beefValidator.New(nodePolicy)}
	}

	return nil
}

type beefValidatorAdapter struct {
	v *beefValidator.BeefValidator
}

func (a *beefValidatorAdapter) ValidateTransaction(ctx context.Context, tx any, feeValOpt validator.FeeValidation, scriptValOpt validator.ScriptValidation) (*bt.Tx, error) {
	beefTx, ok := tx.(*beef.BEEF)
	if !ok {
		return nil, errors.New("invalid tx") // panic maybe?
	}

	return a.v.ValidateTransaction(ctx, beefTx, feeValOpt, scriptValOpt)
}

type defaultValidatorAdapter struct {
	v *defaultValidator.DefaultValidator
}

func (a *defaultValidatorAdapter) ValidateTransaction(ctx context.Context, tx any, feeValOpt validator.FeeValidation, scriptValOpt validator.ScriptValidation) (*bt.Tx, error) {
	btTx, ok := tx.(*bt.Tx)
	if !ok {
		return nil, errors.New("invalid tx") // panic maybe?
	}

	return btTx, a.v.ValidateTransaction(ctx, btTx, feeValOpt, scriptValOpt)
}
