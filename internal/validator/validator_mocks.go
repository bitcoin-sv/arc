package validator

//go:generate moq -pkg mocks -out ./mocks/tx_finder_mock.go . TxFinderI

//go:generate moq -pkg mocks -out ./mocks/merkle_verifier_mock.go . MerkleVerifierI
