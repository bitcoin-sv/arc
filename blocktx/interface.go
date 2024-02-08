package blocktx

import "errors"

var (
	ErrMerklePathNotFoundForTransaction = errors.New("merkle path not found for transaction")
)
