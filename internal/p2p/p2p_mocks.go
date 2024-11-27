package p2p

//go:generate moq -pkg mocks -out ./mocks/peer_mock.go . PeerI
//go:generate moq -pkg mocks -out ./mocks/message_handler_mock.go . MessageHandlerI
