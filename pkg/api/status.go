package api

import (
	"strconv"
)

type StatusCode int

const (
	arcDocServerErrorsUrl = "https://bitcoin-sv.github.io/arc/#/errors?id=_"

	StatusOK                  StatusCode = 200
	ErrStatusBadRequest       StatusCode = 400
	ErrStatusNotFound         StatusCode = 404
	ErrStatusGeneric          StatusCode = 409
	ErrStatusTxFormat         StatusCode = 460
	ErrStatusUnlockingScripts StatusCode = 461
	ErrStatusInputs           StatusCode = 462
	ErrStatusMalformed        StatusCode = 463
	ErrStatusOutputs          StatusCode = 464
	ErrStatusFees             StatusCode = 465
	ErrStatusConflict         StatusCode = 466
	ErrStatusFrozenPolicy     StatusCode = 471
	ErrStatusFrozenConsensus  StatusCode = 472
)

func NewErrorFields(status StatusCode, extraInfo string) *ErrorFields {
	emptyString := ""
	errFields := ErrorFields{
		Status:    int(status),
		ExtraInfo: &emptyString,
	}

	if extraInfo != "" {
		errFields.ExtraInfo = &extraInfo
	}

	switch status {
	case ErrStatusBadRequest:
		errFields.Detail = "The request seems to be malformed and cannot be processed"
		errFields.Title = "Bad request"
		errFields.Type = arcDocServerErrorsUrl + strconv.Itoa(int(ErrStatusBadRequest))
	case ErrStatusNotFound:
		errFields.Detail = "The requested resource could not be found"
		errFields.Title = "Not found"
		errFields.Type = arcDocServerErrorsUrl + strconv.Itoa(int(ErrStatusNotFound))
	case ErrStatusGeneric:
		errFields.Detail = "Transaction could not be processed"
		errFields.Title = "Generic error"
		errFields.Type = arcDocServerErrorsUrl + strconv.Itoa(int(ErrStatusGeneric))
	case ErrStatusTxFormat:
		errFields.Detail = "Transaction is not in extended format, missing input scripts"
		errFields.Title = "Not extended format"
		errFields.Type = arcDocServerErrorsUrl + strconv.Itoa(int(ErrStatusTxFormat))
	case ErrStatusConflict:
		errFields.Detail = "Transaction is valid, but there is a conflicting tx in the block template"
		errFields.Title = "Conflicting tx found"
		errFields.Type = arcDocServerErrorsUrl + strconv.Itoa(int(ErrStatusConflict))
	case ErrStatusUnlockingScripts:
		errFields.Detail = "Transaction is malformed and cannot be processed"
		errFields.Title = "Malformed transaction"
		errFields.Type = arcDocServerErrorsUrl + strconv.Itoa(int(ErrStatusUnlockingScripts))
	case ErrStatusInputs:
		errFields.Detail = "Transaction is invalid because the inputs are non-existent or spent"
		errFields.Title = "Invalid inputs"
		errFields.Type = arcDocServerErrorsUrl + strconv.Itoa(int(ErrStatusInputs))
	case ErrStatusOutputs:
		errFields.Detail = "Transaction is invalid because the outputs are non-existent or invalid"
		errFields.Title = "Invalid outputs"
		errFields.Type = arcDocServerErrorsUrl + strconv.Itoa(int(ErrStatusOutputs))
	case ErrStatusMalformed:
		errFields.Detail = "Transaction is malformed and cannot be processed"
		errFields.Title = "Malformed transaction"
		errFields.Type = arcDocServerErrorsUrl + strconv.Itoa(int(ErrStatusMalformed))
	case ErrStatusFees:
		errFields.Detail = "Fees are too low"
		errFields.Title = "Fee too low"
		errFields.Type = arcDocServerErrorsUrl + strconv.Itoa(int(ErrStatusFees))
	case ErrStatusFrozenPolicy:
		errFields.Detail = "Input Frozen (blacklist manager policy blacklisted)"
		errFields.Title = "Input Frozen"
		errFields.Type = arcDocServerErrorsUrl + strconv.Itoa(int(ErrStatusFrozenPolicy))
	case ErrStatusFrozenConsensus:
		errFields.Detail = "Input Frozen (blacklist manager consensus blacklisted)"
		errFields.Title = "Input Frozen"
		errFields.Type = arcDocServerErrorsUrl + strconv.Itoa(int(ErrStatusFrozenConsensus))
	default:
		errFields.Status = int(ErrStatusGeneric)
		errFields.Detail = "Transaction could not be processed"
		errFields.Title = "Generic error"
		errFields.Type = arcDocServerErrorsUrl + strconv.Itoa(int(ErrStatusGeneric))
	}

	return &errFields
}
