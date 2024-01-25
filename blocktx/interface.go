package blocktx

import "errors"

var (
	ErrMerklePathNotFoundForTransaction = errors.New("Merkle path not found for transaction")
)
