package p2p

//go:generate moq -pkg mocks -out ./mocks/peer_mock.go . PeerI

//go:generate moq -pkg mocks -out ./mocks/message_handler_mock.go . MessageHandlerI

//go:generate moq -pkg mocks -out ./mocks/wire_msg_mock.go . Message

//go:generate moq -pkg mocks -out ./mocks/dialer_mock.go . Dialer
