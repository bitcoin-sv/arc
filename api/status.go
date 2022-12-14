package api

import (
	"strconv"
)

const ArcDocServerUrl = "https://arc.bitcoinsv.com"

const (
	StatusAlreadyMined       = 200
	StatusAlreadyInMempool   = 200
	StatusAddedBlockTemplate = 201
)

type ErrorCode int

// custom (http) status code
const (
	ErrStatusBadRequest       ErrorCode = 400
	ErrStatusNotFound         ErrorCode = 404
	ErrStatusGeneric          ErrorCode = 409
	ErrStatusTxFormat         ErrorCode = 460
	ErrStatusUnlockingScripts ErrorCode = 461
	ErrStatusInputs           ErrorCode = 462
	ErrStatusOutputs          ErrorCode = 463
	ErrStatusMalformed        ErrorCode = 464
	ErrStatusFees             ErrorCode = 465
	ErrStatusConflict         ErrorCode = 466
	ErrStatusFrozenPolicy     ErrorCode = 481
	ErrStatusFrozenConsensus  ErrorCode = 482
)

var (
	ErrBadRequest = ErrorFields{
		Detail: "The request seems to be malformed and cannot be processed",
		Status: int(ErrStatusBadRequest), // TODO check if we can check ErrStatus from generated code from yaml
		Title:  "Bad request",
		Type:   ArcDocServerUrl + "/errors/" + strconv.Itoa(int(ErrStatusBadRequest)),
	}

	ErrNotFound = ErrorFields{
		Detail: "The requested resource could not be found",
		Status: int(ErrStatusNotFound),
		Title:  "Not found",
		Type:   ArcDocServerUrl + "/errors/" + strconv.Itoa(int(ErrStatusNotFound)),
	}

	ErrGeneric = ErrorFields{
		Detail: "Transaction could not be processed",
		Status: int(ErrStatusGeneric),
		Title:  "Generic error",
		Type:   ArcDocServerUrl + "/errors/" + strconv.Itoa(int(ErrStatusGeneric)),
	}

	ErrTxFormat = ErrorFields{
		Detail: "Transaction is not in extended format, missing input scripts",
		Status: int(ErrStatusTxFormat),
		Title:  "Not extended format",
		Type:   ArcDocServerUrl + "/errors/" + strconv.Itoa(int(ErrStatusTxFormat)),
	}

	ErrConflict = ErrorFields{
		Detail: "Transaction is valid, but there is a conflicting tx in the block template",
		Status: int(ErrStatusConflict),
		Title:  "Conflicting tx found",
		Type:   ArcDocServerUrl + "/errors/" + strconv.Itoa(int(ErrStatusConflict)),
	}

	ErrUnlockingScripts = ErrorFields{
		Detail: "Transaction is malformed and cannot be processed",
		Status: int(ErrStatusUnlockingScripts),
		Title:  "Malformed transaction",
		Type:   ArcDocServerUrl + "/errors/" + strconv.Itoa(int(ErrStatusUnlockingScripts)),
	}

	ErrInputs = ErrorFields{
		Detail: "Transaction is invalid because the inputs are non-existent or spent",
		Status: int(ErrStatusInputs),
		Title:  "Invalid inputs",
		Type:   ArcDocServerUrl + "/errors/" + strconv.Itoa(int(ErrStatusInputs)),
	}

	ErrOutputs = ErrorFields{
		Detail: "Transaction is invalid because the outputs are non-existent or invalid",
		Status: int(ErrStatusOutputs),
		Title:  "Invalid outputs",
		Type:   ArcDocServerUrl + "/errors/" + strconv.Itoa(int(ErrStatusOutputs)),
	}

	ErrMalformed = ErrorFields{
		Detail: "Transaction is malformed and cannot be processed",
		Status: int(ErrStatusMalformed),
		Title:  "Malformed transaction",
		Type:   ArcDocServerUrl + "/errors/" + strconv.Itoa(int(ErrStatusMalformed)),
	}

	ErrFees = ErrorFields{
		Detail: "Fees are too low",
		Status: int(ErrStatusFees),
		Title:  "Fee too low",
		Type:   ArcDocServerUrl + "/errors/" + strconv.Itoa(int(ErrStatusFees)),
	}

	ErrFrozenPolicy = ErrorFields{
		Detail: "Input Frozen (blacklist manager policy blacklisted)",
		Status: int(ErrStatusFrozenPolicy),
		Title:  "Input Frozen",
		Type:   ArcDocServerUrl + "/errors/" + strconv.Itoa(int(ErrStatusFrozenPolicy)),
	}

	ErrFrozenConsensus = ErrorFields{
		Detail: "Input Frozen (blacklist manager consensus blacklisted)",
		Status: int(ErrStatusFrozenConsensus),
		Title:  "Input Frozen",
		Type:   ArcDocServerUrl + "/errors/" + strconv.Itoa(int(ErrStatusFrozenConsensus)),
	}
)

var ErrByStatus = map[ErrorCode]*ErrorFields{
	ErrStatusNotFound:         &ErrNotFound,
	ErrStatusBadRequest:       &ErrBadRequest,
	ErrStatusGeneric:          &ErrGeneric,
	ErrStatusTxFormat:         &ErrTxFormat,
	ErrStatusConflict:         &ErrConflict,
	ErrStatusUnlockingScripts: &ErrUnlockingScripts,
	ErrStatusInputs:           &ErrInputs,
	ErrStatusMalformed:        &ErrMalformed,
	ErrStatusFees:             &ErrFees,
	ErrStatusFrozenPolicy:     &ErrFrozenPolicy,
	ErrStatusFrozenConsensus:  &ErrFrozenConsensus,
}
