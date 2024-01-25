package blocktx

import "errors"

var (
	// ErrBlockNotFound is returned when a block is not found.
	ErrBlockNotFound = errors.New("block not found")

	// ErrTransactionNotFound is returned when a transaction is not found.
	ErrTransactionNotFound = errors.New("transaction not found")

	ErrTransactionNotFoundForMerklePath = errors.New("transaction not found for given Merkle path")
)
