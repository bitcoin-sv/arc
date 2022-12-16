package api

import (
	"strconv"
)

const ArcDocServerUrl = "https://arc.bitcoinsv.com"

type StatusCode int

const (
	StatusAlreadyMined       StatusCode = 200
	StatusAlreadyInMempool   StatusCode = 200
	StatusAddedBlockTemplate StatusCode = 201
)

// custom (http) status code
const (
	ErrStatusBadRequest       StatusCode = 400
	ErrStatusNotFound         StatusCode = 404
	ErrStatusGeneric          StatusCode = 409
	ErrStatusTxFormat         StatusCode = 460
	ErrStatusUnlockingScripts StatusCode = 461
	ErrStatusInputs           StatusCode = 462
	ErrStatusOutputs          StatusCode = 463
	ErrStatusMalformed        StatusCode = 464
	ErrStatusFees             StatusCode = 465
	ErrStatusConflict         StatusCode = 466
	ErrStatusFrozenPolicy     StatusCode = 481
	ErrStatusFrozenConsensus  StatusCode = 482
)

var (
	StatusText = map[StatusCode]string{
		StatusAlreadyInMempool:   "Already in mempool",
		StatusAddedBlockTemplate: "Added to block template",
	}
)

var (
	ErrBadRequest = ErrorFields{
		Detail: "The request seems to be malformed and cannot be processed",
		Status: int(ErrStatusBadRequest),
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

var ErrByStatus = map[StatusCode]*ErrorFields{
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
