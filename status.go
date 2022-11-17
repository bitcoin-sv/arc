package mapi

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
	ErrStatusConflict         ErrStatus = 460
	ErrStatusUnlockingScripts ErrStatus = 461
	ErrStatusInputs           ErrStatus = 462
	ErrStatusMalformed        ErrStatus = 463
	ErrStatusFees             ErrStatus = 464
	ErrStatusFrozenPolicy     ErrStatus = 471
	ErrStatusFrozenConsensus  ErrStatus = 472
)

var (
	ErrBadRequest = ErrorFields{
		Detail: "The request seems to be malformed and cannot be processed",
		Status: int(ErrStatusBadRequest), // TODO check if we can check ErrStatus from generated code from yaml
		Title:  "Bad request",
		Type:   MapiDocServerUrl + "/errors/400",
	}

	ErrNotFound = ErrorFields{
		Detail: "The requested resource could not be found",
		Status: int(ErrStatusNotFound),
		Title:  "Not found",
		Type:   MapiDocServerUrl + "/errors/404",
	}

	ErrGeneric = ErrorFields{
		Detail: "Transaction could not be processed",
		Status: int(ErrStatusGeneric),
		Title:  "Generic error",
		Type:   MapiDocServerUrl + "/errors/409",
	}

	ErrConflict = ErrorFields{
		Detail: "Transaction is valid, but there is a conflicting tx in the block template",
		Status: int(ErrStatusConflict),
		Title:  "Conflicting tx found",
		Type:   MapiDocServerUrl + "/errors/460",
	}

	ErrUnlockingScripts = ErrorFields{
		Detail: "Transaction is malformed and cannot be processed",
		Status: int(ErrStatusUnlockingScripts),
		Title:  "Malformed transaction",
		Type:   MapiDocServerUrl + "/errors/461",
	}

	ErrInputs = ErrorFields{
		Detail: "Transaction is invalid because the inputs are non-existent or spent",
		Status: int(ErrStatusInputs),
		Title:  "Invalid inputs",
		Type:   MapiDocServerUrl + "/errors/462",
	}

	ErrMalformed = ErrorFields{
		Detail: "Transaction is malformed and cannot be processed",
		Status: int(ErrStatusMalformed),
		Title:  "Malformed transaction",
		Type:   MapiDocServerUrl + "/errors/463",
	}

	ErrFees = ErrorFields{
		Detail: "Fees are too low",
		Status: int(ErrStatusFees),
		Title:  "Fee too low",
		Type:   MapiDocServerUrl + "/errors/464",
	}

	ErrFrozenPolicy = ErrorFields{
		Detail: "Input Frozen (blacklist manager policy blacklisted)",
		Status: int(ErrStatusFrozenPolicy),
		Title:  "Input Frozen",
		Type:   MapiDocServerUrl + "/errors/471",
	}

	ErrFrozenConsensus = ErrorFields{
		Detail: "Input Frozen (blacklist manager consensus blacklisted)",
		Status: int(ErrStatusFrozenConsensus),
		Title:  "Input Frozen",
		Type:   MapiDocServerUrl + "/errors/471",
	}
)

var ErrByStatus = map[ErrStatus]*ErrorFields{
	ErrStatusNotFound:         &ErrNotFound,
	ErrStatusBadRequest:       &ErrBadRequest,
	ErrStatusGeneric:          &ErrGeneric,
	ErrStatusConflict:         &ErrConflict,
	ErrStatusUnlockingScripts: &ErrUnlockingScripts,
	ErrStatusInputs:           &ErrInputs,
	ErrStatusMalformed:        &ErrMalformed,
	ErrStatusFees:             &ErrFees,
	ErrStatusFrozenPolicy:     &ErrFrozenPolicy,
	ErrStatusFrozenConsensus:  &ErrFrozenConsensus,
}
