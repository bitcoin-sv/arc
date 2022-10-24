package handler

import "github.com/taal/mapi"

const MapiDocServerUrl = "https://mapi.bitcoinsv.com"

// custom http status code
var (
	// StatusMalformed is thrown when a transaction cannot be processed by go-bt
	StatusMalformed = 463
)

var (
	ErrBadRequest = mapi.ErrorFields{
		Detail: "The request seems to be malformed and cannot be processed",
		Status: 400,
		Title:  "Bad request",
		Type:   MapiDocServerUrl + "/errors/400",
	}

	ErrorConflict = mapi.ErrorFields{
		Detail: "Transaction could not be processed",
		Status: 409,
		Title:  "Generic error",
		Type:   MapiDocServerUrl + "/errors/461",
	}

	ErrGeneric = mapi.ErrorFields{
		Detail: "Transaction is valid, but there is a conflicting tx in the block template",
		Status: 460,
		Title:  "Conflicting tx found",
		Type:   MapiDocServerUrl + "/errors/461",
	}

	ErrorUnlockingScripts = mapi.ErrorFields{
		Detail: "Transaction is malformed and cannot be processed",
		Status: 461,
		Title:  "Malformed transaction",
		Type:   MapiDocServerUrl + "/errors/461",
	}

	ErrorInputs = mapi.ErrorFields{
		Detail: "Transaction is invalid because the inputs are non-existent or spent",
		Status: 462,
		Title:  "Invalid inputs",
		Type:   MapiDocServerUrl + "/errors/462",
	}

	ErrMalformed = mapi.ErrorFields{
		Detail: "Transaction is malformed and cannot be processed",
		Status: 463,
		Title:  "Malformed transaction",
		Type:   MapiDocServerUrl + "/errors/463",
	}

	ErrorFrozenPolicy = mapi.ErrorFields{
		Detail: "Input Frozen (blacklist manager policy blacklisted)",
		Status: 471,
		Title:  "Input Frozen",
		Type:   MapiDocServerUrl + "/errors/471",
	}

	ErrorFrozenConsensus = mapi.ErrorFields{
		Detail: "Input Frozen (blacklist manager consensus blacklisted)",
		Status: 472,
		Title:  "Input Frozen",
		Type:   MapiDocServerUrl + "/errors/471",
	}
)

var ErrByStatus = map[uint]*mapi.ErrorFields{
	400: &ErrBadRequest,
	409: &ErrorConflict,
	460: &ErrGeneric,
	461: &ErrorUnlockingScripts,
	462: &ErrorInputs,
	463: &ErrMalformed,
	471: &ErrorFrozenPolicy,
	472: &ErrorFrozenConsensus,
}
