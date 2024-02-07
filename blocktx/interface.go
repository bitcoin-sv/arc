package blocktx

import "github.com/pkg/errors"

var (
	ErrMerklePathNotFoundForTransaction = errors.New("Merkle path not found for transaction")
)
