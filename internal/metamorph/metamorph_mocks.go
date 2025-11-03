package metamorph

// from ./metamorph_api/
//go:generate moq -pkg mocks -out ./mocks/metamorph_api_mock.go ./metamorph_api MetaMorphAPIClient

// from health_check.go
//go:generate moq -pkg mocks -out ./mocks/health_watch_server_mock.go . HealthWatchServer

// from processor.go
//go:generate moq -pkg mocks -out ./mocks/mediator_mock.go . Mediator

// from zmq.go
//go:generate moq -pkg mocks -out ./mocks/zmq_mock.go . ZMQI

// from server.go
//go:generate moq -pkg mocks -out ./mocks/processor_mock.go . ProcessorI
//go:generate moq -pkg mocks -out ./mocks/bitcoin_mock.go . BitcoinNode
