package api

import (
	"strconv"

	"go.opentelemetry.io/otel/attribute"
)

type StatusCode int

const (
	arcDocServerErrorsURL = "https://bitcoin-sv.github.io/arc/#/errors?id=_"

	StatusOK                              StatusCode = 200
	ErrStatusBadRequest                   StatusCode = 400
	ErrStatusNotFound                     StatusCode = 404
	ErrStatusGeneric                      StatusCode = 409
	ErrStatusTxFormat                     StatusCode = 460
	ErrStatusUnlockingScripts             StatusCode = 461
	ErrStatusInputs                       StatusCode = 462
	ErrStatusMalformed                    StatusCode = 463
	ErrStatusOutputs                      StatusCode = 464
	ErrStatusFees                         StatusCode = 465
	ErrStatusConflict                     StatusCode = 466
	ErrStatusMinedAncestorsNotFound       StatusCode = 467
	ErrStatusCalculatingMerkleRoots       StatusCode = 468
	ErrStatusValidatingMerkleRoots        StatusCode = 469
	ErrStatusFrozenPolicy                 StatusCode = 471
	ErrStatusFrozenConsensus              StatusCode = 472
	ErrStatusCumulativeFees               StatusCode = 473
	ErrStatusTxSize                       StatusCode = 474
	ErrStatusMinedAncestorsNotFoundInBUMP StatusCode = 475
)

func (e *ErrorFields) GetSpanAttributes() []attribute.KeyValue {
	attr := []attribute.KeyValue{attribute.Int("code", e.Status)}
	if e.ExtraInfo != nil {
		attr = append(attr, attribute.String("extraInfo", *e.ExtraInfo))
	}
	return attr
}

func NewErrorFields(status StatusCode, extraInfo string) *ErrorFields {
	emptyString := ""
	errFields := ErrorFields{
		Status:    int(status),
		ExtraInfo: &emptyString,
	}

	if extraInfo != "" {
		errFields.ExtraInfo = &extraInfo
	}

	var statusList = map[StatusCode]*ErrorFields{
		ErrStatusBadRequest: { // 400
			Detail: "The request seems to be malformed and cannot be processed",
			Title:  "Bad request",
			Type:   arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusBadRequest)),
		},
		ErrStatusNotFound: { // 404
			Detail: "The requested resource could not be found",
			Title:  "Not found",
			Type:   arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusNotFound)),
		},
		ErrStatusGeneric: { // 409
			Detail: "Transaction could not be processed",
			Title:  "Generic error",
			Type:   arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusGeneric)),
		},
		ErrStatusTxFormat: { // 460
			Detail: "Missing input scripts: Transaction could not be transformed to extended format",
			Title:  "Not extended format",
			Type:   arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusTxFormat)),
		},
		ErrStatusUnlockingScripts: { // 461
			Detail: "Transaction is malformed and cannot be processed",
			Title:  "Malformed transaction",
			Type:   arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusUnlockingScripts)),
		},
		ErrStatusInputs: { // 462
			Detail: "Transaction is invalid because the inputs are non-existent or spent",
			Title:  "Invalid inputs",
			Type:   arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusInputs)),
		},
		ErrStatusMalformed: { // 463
			Detail: "Transaction is malformed and cannot be processed",
			Title:  "Malformed transaction",
			Type:   arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusMalformed)),
		},
		ErrStatusOutputs: { // 464
			Detail: "Transaction is invalid because the outputs are non-existent or invalid",
			Title:  "Invalid outputs",
			Type:   arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusOutputs)),
		},
		ErrStatusFees: { // 465
			Detail: "Fees are insufficient",
			Title:  "Fee too low",
			Type:   arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusFees)),
		},
		ErrStatusConflict: { // 466
			Detail: "Transaction is valid, but there is a conflicting tx in the block template",
			Title:  "Conflicting tx found",
			Type:   arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusConflict)),
		},
		ErrStatusMinedAncestorsNotFound: { // 467
			Detail: "BEEF validation failed: couldn't find mined ancestor of the transaction in provided beef transactions",
			Title:  "Mined ancestors not found",
			Type:   arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusMinedAncestorsNotFound)),
		},
		ErrStatusCalculatingMerkleRoots: { // 468
			Detail: "BEEF validation failed: couldn't calculate Merkle Roots from given BUMPs",
			Title:  "Invalid BUMPs",
			Type:   arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusCalculatingMerkleRoots)),
		},
		ErrStatusValidatingMerkleRoots: { // 469
			Detail: "BEEF validation failed: couldn't validate Merkle Roots",
			Title:  "Merkle Roots validation failed",
			Type:   arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusValidatingMerkleRoots)),
		},
		ErrStatusFrozenPolicy: { // 471
			Detail: "Input Frozen (blacklist manager policy blacklisted)",
			Title:  "Input Frozen",
			Type:   arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusFrozenPolicy)),
		},
		ErrStatusFrozenConsensus: { // 472
			Detail: "Input Frozen (blacklist manager consensus blacklisted)",
			Title:  "Input Frozen",
			Type:   arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusFrozenConsensus)),
		},
		ErrStatusCumulativeFees: { // 473
			Detail: "Cumulative fee validation failed",
			Title:  "Cumulative fee validation failed",
			Type:   arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusCumulativeFees)),
		},
		ErrStatusTxSize: { // 474
			Detail: "Transaction size validation failed",
			Title:  "Transaction size validation failed",
			Type:   arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusTxSize)),
		},
		ErrStatusMinedAncestorsNotFoundInBUMP: { // 475
			Detail: "BEEF validation failed: couldn't find mined ancestor of the transaction in provided BUMPs",
			Title:  "Mined ancestors not found in BUMPs",
			Type:   arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusMinedAncestorsNotFoundInBUMP)),
		},
	}
	errInfo, ok := statusList[status]
	if !ok {
		errFields.Status = int(ErrStatusGeneric)
		errFields.Detail = "Transaction could not be processed"
		errFields.Title = "Generic error"
		errFields.Type = arcDocServerErrorsURL + strconv.Itoa(int(ErrStatusGeneric))
	} else {
		errFields.Detail = errInfo.Detail
		errFields.Title = errInfo.Title
		errFields.Type = errInfo.Type
	}

	return &errFields
}
