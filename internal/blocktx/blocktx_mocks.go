package blocktx

// from ./blocktx_api
//go:generate moq -pkg mocks -out ./mocks/blocktx_api_mock.go ./blocktx_api BlockTxAPIClient

// from health_check.go
//go:generate moq -pkg mocks -out ./mocks/health_watch_server_mock.go . HealthWatchServer

// from nats_core_client.go
//go:generate moq -pkg mocks -out ./mocks/mq_client_mock.go . MessageQueueClient

// from client.go
//go:generate moq -pkg mocks -out ./mocks/merkle_roots_verifier_mock.go . MerkleRootsVerifier

// from client.go
//go:generate moq -pkg mocks -out ./mocks/blocktx_client_mock.go . Watcher
