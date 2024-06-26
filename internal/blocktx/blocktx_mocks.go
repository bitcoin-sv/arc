package blocktx

// from health_check.go
//go:generate moq -pkg mocks -out ./mocks/health_watch_server_mock.go . HealthWatchServer

// from mq_client.go
//go:generate moq -pkg mocks -out ./mocks/mq_client_mock.go . MessageQueueClient

// from peer.go
//go:generate moq -pkg mocks -out ./mocks/peer_mock.go . Peer

// from peer_manager.go
//go:generate moq -pkg mocks -out ./mocks/peer_manager_mock.go . PeerManager
