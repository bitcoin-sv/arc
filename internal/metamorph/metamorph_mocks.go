package metamorph

// from ./metamorph_api/
//go:generate moq -pkg mocks -out ./mocks/metamorph_api_mock.go ./metamorph_api MetaMorphAPIClient

// from health_check.go
//go:generate moq -pkg mocks -out ./mocks/health_watch_server_mock.go . HealthWatchServer

// from nats_core_client.go
//go:generate moq -pkg mocks -out ./mocks/message_queue_mock.go . MessageQueueClient

// from processor.go
//go:generate moq -pkg mocks -out ./mocks/callback_sender_mock.go . CallbackSender

// from zmq.go
//go:generate moq -pkg mocks -out ./mocks/zmq_mock.go . ZMQI

// from server.go
//go:generate moq -pkg mocks -out ./mocks/processor_mock.go . ProcessorI
//go:generate moq -pkg mocks -out ./mocks/bitcon_mock.go . BitcoinNode
