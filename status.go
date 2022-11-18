package mapi

import "strconv"

const MapiDocServerUrl = "https://mapi.bitcoinsv.com"

const (
	StatusAlreadyMined       = 200
	StatusAlreadyInMempool   = 200
	StatusAddedBlockTemplate = 201
)

type ErrStatus int

// custom (http) status code
const (
	ErrStatusBadRequest       ErrStatus = 400
	ErrStatusNotFound         ErrStatus = 404
	ErrStatusGeneric          ErrStatus = 409
	ErrStatusTxFormat         ErrStatus = 460
	ErrStatusUnlockingScripts ErrStatus = 461
	ErrStatusInputs           ErrStatus = 462
	ErrStatusOutputs          ErrStatus = 463
	ErrStatusMalformed        ErrStatus = 464
	ErrStatusFees             ErrStatus = 465
	ErrStatusConflict         ErrStatus = 466
	ErrStatusFrozenPolicy     ErrStatus = 481
	ErrStatusFrozenConsensus  ErrStatus = 482
)

var (
	ErrBadRequest = ErrorFields{
		Detail: "The request seems to be malformed and cannot be processed",
		Status: int(ErrStatusBadRequest), // TODO check if we can check ErrStatus from generated code from yaml
		Title:  "Bad request",
		Type:   MapiDocServerUrl + "/errors/" + strconv.Itoa(int(ErrStatusBadRequest)),
	}

	ErrNotFound = ErrorFields{
		Detail: "The requested resource could not be found",
		Status: int(ErrStatusNotFound),
		Title:  "Not found",
		Type:   MapiDocServerUrl + "/errors/" + strconv.Itoa(int(ErrStatusNotFound)),
	}

	ErrGeneric = ErrorFields{
		Detail: "Transaction could not be processed",
		Status: int(ErrStatusGeneric),
		Title:  "Generic error",
		Type:   MapiDocServerUrl + "/errors/" + strconv.Itoa(int(ErrStatusGeneric)),
	}

	ErrTxFormat = ErrorFields{
		Detail: "Transaction is not in extended format, missing input scripts",
		Status: int(ErrStatusTxFormat),
		Title:  "Not extended format",
		Type:   MapiDocServerUrl + "/errors/" + strconv.Itoa(int(ErrStatusTxFormat)),
	}

	ErrConflict = ErrorFields{
		Detail: "Transaction is valid, but there is a conflicting tx in the block template",
		Status: int(ErrStatusConflict),
		Title:  "Conflicting tx found",
		Type:   MapiDocServerUrl + "/errors/" + strconv.Itoa(int(ErrStatusConflict)),
	}

	ErrUnlockingScripts = ErrorFields{
		Detail: "Transaction is malformed and cannot be processed",
		Status: int(ErrStatusUnlockingScripts),
		Title:  "Malformed transaction",
		Type:   MapiDocServerUrl + "/errors/" + strconv.Itoa(int(ErrStatusUnlockingScripts)),
	}

	ErrInputs = ErrorFields{
		Detail: "Transaction is invalid because the inputs are non-existent or spent",
		Status: int(ErrStatusInputs),
		Title:  "Invalid inputs",
		Type:   MapiDocServerUrl + "/errors/" + strconv.Itoa(int(ErrStatusInputs)),
	}

	ErrOutputs = ErrorFields{
		Detail: "Transaction is invalid because the outputs are non-existent or invalid",
		Status: int(ErrStatusOutputs),
		Title:  "Invalid outputs",
		Type:   MapiDocServerUrl + "/errors/" + strconv.Itoa(int(ErrStatusOutputs)),
	}

	ErrMalformed = ErrorFields{
		Detail: "Transaction is malformed and cannot be processed",
		Status: int(ErrStatusMalformed),
		Title:  "Malformed transaction",
		Type:   MapiDocServerUrl + "/errors/" + strconv.Itoa(int(ErrStatusMalformed)),
	}

	ErrFees = ErrorFields{
		Detail: "Fees are too low",
		Status: int(ErrStatusFees),
		Title:  "Fee too low",
		Type:   MapiDocServerUrl + "/errors/" + strconv.Itoa(int(ErrStatusFees)),
	}

	ErrFrozenPolicy = ErrorFields{
		Detail: "Input Frozen (blacklist manager policy blacklisted)",
		Status: int(ErrStatusFrozenPolicy),
		Title:  "Input Frozen",
		Type:   MapiDocServerUrl + "/errors/" + strconv.Itoa(int(ErrStatusFrozenPolicy)),
	}

	ErrFrozenConsensus = ErrorFields{
		Detail: "Input Frozen (blacklist manager consensus blacklisted)",
		Status: int(ErrStatusFrozenConsensus),
		Title:  "Input Frozen",
		Type:   MapiDocServerUrl + "/errors/" + strconv.Itoa(int(ErrStatusFrozenConsensus)),
	}
)

var ErrByStatus = map[ErrStatus]*ErrorFields{
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
