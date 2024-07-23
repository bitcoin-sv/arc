package blocktx

//go:generate moq -pkg mocks -out ./mocks/merkle_roots_verifier_mock.go . MerkleRootsVerifier

//go:generate moq -pkg mocks -out ./mocks/blocktx_client_mock.go . BlocktxClient
