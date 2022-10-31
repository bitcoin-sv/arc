package mapi

const MapiDocServerUrl = "https://mapi.bitcoinsv.com"

var (
	StatusAlreadyMined       = 200
	StatusAlreadyInMempool   = 200
	StatusAddedBlockTemplate = 201
)

// custom (http) status code
var (
	ErrStatusBadRequest       = 400
	ErrStatusGeneric          = 409
	ErrStatusConflict         = 460
	ErrStatusUnlockingScripts = 461
	ErrStatusInputs           = 462
	ErrStatusMalformed        = 463
	ErrStatusFrozenPolicy     = 471
	ErrStatusFrozenConsensus  = 472
)

var (
	ErrBadRequest = ErrorFields{
		Detail: "The request seems to be malformed and cannot be processed",
		Status: ErrStatusBadRequest,
		Title:  "Bad request",
		Type:   MapiDocServerUrl + "/errors/400",
	}

	ErrGeneric = ErrorFields{
		Detail: "Transaction could not be processed",
		Status: ErrStatusGeneric,
		Title:  "Generic error",
		Type:   MapiDocServerUrl + "/errors/409",
	}

	ErrConflict = ErrorFields{
		Detail: "Transaction is valid, but there is a conflicting tx in the block template",
		Status: ErrStatusConflict,
		Title:  "Conflicting tx found",
		Type:   MapiDocServerUrl + "/errors/460",
	}

	ErrUnlockingScripts = ErrorFields{
		Detail: "Transaction is malformed and cannot be processed",
		Status: ErrStatusUnlockingScripts,
		Title:  "Malformed transaction",
		Type:   MapiDocServerUrl + "/errors/461",
	}

	ErrInputs = ErrorFields{
		Detail: "Transaction is invalid because the inputs are non-existent or spent",
		Status: ErrStatusInputs,
		Title:  "Invalid inputs",
		Type:   MapiDocServerUrl + "/errors/462",
	}

	ErrMalformed = ErrorFields{
		Detail: "Transaction is malformed and cannot be processed",
		Status: ErrStatusMalformed,
		Title:  "Malformed transaction",
		Type:   MapiDocServerUrl + "/errors/463",
	}

	ErrFrozenPolicy = ErrorFields{
		Detail: "Input Frozen (blacklist manager policy blacklisted)",
		Status: ErrStatusFrozenPolicy,
		Title:  "Input Frozen",
		Type:   MapiDocServerUrl + "/errors/471",
	}

	ErrFrozenConsensus = ErrorFields{
		Detail: "Input Frozen (blacklist manager consensus blacklisted)",
		Status: ErrStatusFrozenConsensus,
		Title:  "Input Frozen",
		Type:   MapiDocServerUrl + "/errors/471",
	}
)

var ErrByStatus = map[int]*ErrorFields{
	ErrStatusBadRequest:       &ErrBadRequest,
	ErrStatusGeneric:          &ErrGeneric,
	ErrStatusConflict:         &ErrConflict,
	ErrStatusUnlockingScripts: &ErrUnlockingScripts,
	ErrStatusInputs:           &ErrInputs,
	ErrStatusMalformed:        &ErrMalformed,
	ErrStatusFrozenPolicy:     &ErrFrozenPolicy,
	ErrStatusFrozenConsensus:  &ErrFrozenConsensus,
}
