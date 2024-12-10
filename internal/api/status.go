package api

import (
	"github.com/bitcoin-sv/arc/pkg/api"
	"strconv"

	"go.opentelemetry.io/otel/attribute"
)

type StatusCode int

const (
	arcDocServerErrorsURL = "https://bitcoin-sv.github.io/arc/#/errors?id=_"

	StatusOK                        StatusCode = 200
	ErrStatusBadRequest             StatusCode = 400
	ErrStatusNotFound               StatusCode = 404
	ErrStatusGeneric                StatusCode = 409
	ErrStatusTxFormat               StatusCode = 460
	ErrStatusUnlockingScripts       StatusCode = 461
	ErrStatusInputs                 StatusCode = 462
	ErrStatusMalformed              StatusCode = 463
	ErrStatusOutputs                StatusCode = 464
	ErrStatusFees                   StatusCode = 465
	ErrStatusConflict               StatusCode = 466
	ErrStatusMinedAncestorsNotFound StatusCode = 467
	ErrStatusCalculatingMerkleRoots StatusCode = 468
	ErrStatusValidatingMerkleRoots  StatusCode = 469
	ErrStatusFrozenPolicy           StatusCode = 471
	ErrStatusFrozenConsensus        StatusCode = 472
	ErrStatusCumulativeFees         StatusCode = 473
	ErrStatusTxSize                 StatusCode = 474
)

func (e *api.ErrorFields) GetSpanAttributes() []attribute.KeyValue {
	attr := []attribute.KeyValue{attribute.Int("code", e.Status)}
	if e.ExtraInfo != nil {
		attr = append(attr, attribute.String("extraInfo", *e.ExtraInfo))
	}
	return attr
}

func NewErrorFields(status StatusCode, extraInfo string) *api.ErrorFields {
	emptyString := ""
	errFields := api.ErrorFields{
		Status:    int(status),
		ExtraInfo: &emptyString,
	}

	if extraInfo != "" {
		errFields.ExtraInfo = &extraInfo
	}

	switch status {
	case ErrStatusBadRequest: // 400
		errFields.Detail = "The request seems to be malformed and cannot be processed"
		errFields.Title = "Bad request"
		errFields.Type = arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusBadRequest))
	case ErrStatusNotFound: // 404
		errFields.Detail = "The requested resource could not be found"
		errFields.Title = "Not found"
		errFields.Type = arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusNotFound))
	case ErrStatusGeneric: // 409
		errFields.Detail = "Transaction could not be processed"
		errFields.Title = "Generic error"
		errFields.Type = arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusGeneric))
	case ErrStatusTxFormat: // 460
		errFields.Detail = "Missing input scripts: Transaction could not be transformed to extended format"
		errFields.Title = "Not extended format"
		errFields.Type = arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusTxFormat))
	case ErrStatusUnlockingScripts: // 461
		errFields.Detail = "Transaction is malformed and cannot be processed"
		errFields.Title = "Malformed transaction"
		errFields.Type = arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusUnlockingScripts))
	case ErrStatusInputs: // 462
		errFields.Detail = "Transaction is invalid because the inputs are non-existent or spent"
		errFields.Title = "Invalid inputs"
		errFields.Type = arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusInputs))
	case ErrStatusMalformed: // 463
		errFields.Detail = "Transaction is malformed and cannot be processed"
		errFields.Title = "Malformed transaction"
		errFields.Type = arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusMalformed))
	case ErrStatusOutputs: // 464
		errFields.Detail = "Transaction is invalid because the outputs are non-existent or invalid"
		errFields.Title = "Invalid outputs"
		errFields.Type = arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusOutputs))
	case ErrStatusFees: // 465
		errFields.Detail = "Fees are insufficient"
		errFields.Title = "Fee too low"
		errFields.Type = arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusFees))
	case ErrStatusConflict: // 466
		errFields.Detail = "Transaction is valid, but there is a conflicting tx in the block template"
		errFields.Title = "Conflicting tx found"
		errFields.Type = arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusConflict))
	case ErrStatusMinedAncestorsNotFound: // 467
		errFields.Detail = "BEEF validation failed: couldn't find mined ancestor of the transaction"
		errFields.Title = "Mined ancestors not found"
		errFields.Type = arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusMinedAncestorsNotFound))
	case ErrStatusCalculatingMerkleRoots: // 468
		errFields.Detail = "BEEF validation failed: couldn't calculate Merkle Roots from given BUMPs"
		errFields.Title = "Invalid BUMPs"
		errFields.Type = arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusCalculatingMerkleRoots))
	case ErrStatusValidatingMerkleRoots: // 469
		errFields.Detail = "BEEF validation failed: couldn't validate Merkle Roots"
		errFields.Title = "Merkle Roots validation failed"
		errFields.Type = arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusValidatingMerkleRoots))
	case ErrStatusFrozenPolicy: // 471
		errFields.Detail = "Input Frozen (blacklist manager policy blacklisted)"
		errFields.Title = "Input Frozen"
		errFields.Type = arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusFrozenPolicy))
	case ErrStatusFrozenConsensus: // 472
		errFields.Detail = "Input Frozen (blacklist manager consensus blacklisted)"
		errFields.Title = "Input Frozen"
		errFields.Type = arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusFrozenConsensus))
	case ErrStatusCumulativeFees: // 473
		errFields.Detail = "Cumulative fee validation failed"
		errFields.Title = "Cumulative fee validation failed"
		errFields.Type = arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusCumulativeFees))
	case ErrStatusTxSize: // 474
		errFields.Detail = "Transaction size validation failed"
		errFields.Title = "Transaction size validation failed"
		errFields.Type = arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusTxSize))
	default:
		errFields.Status = int(ErrStatusGeneric)
		errFields.Detail = "Transaction could not be processed"
		errFields.Title = "Generic error"
		errFields.Type = arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusGeneric))
	}

	return &errFields
}
