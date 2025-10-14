package blocktx

// from ./blocktx_api
//go:generate moq -pkg mocks -out ./mocks/blocktx_api_mock.go ./blocktx_api BlockTxAPIClient

// from health_check.go
//go:generate moq -pkg mocks -out ./mocks/health_watch_server_mock.go . HealthWatchServer

// from server.go
//go:generate moq -pkg mocks -out ./mocks/blocktx_processor_mock.go . ProcessorI
//go:generate moq -pkg mocks -out ./mocks/blocktx_peer_manager_mock.go . PeerManager
