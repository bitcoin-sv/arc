package blocktx

// from ./blocktx_api
//go:generate moq -pkg mocks -out ./mocks/blocktx_api_mock.go ./blocktx_api BlockTxAPIClient

// from nats_core_client.go
//go:generate moq -pkg mocks -out ./mocks/mq_client_mock.go ../mq MessageQueueClient

// from client.go
//go:generate moq -pkg mocks -out ./mocks/merkle_roots_verifier_mock.go . MerkleRootsVerifier
//go:generate moq -pkg mocks -out ./mocks/blocktx_client_mock.go . Client
