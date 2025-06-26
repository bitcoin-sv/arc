package api

//go:generate moq -pkg mocks -out ./mocks/client_interface_mock.go ../../pkg/api ClientInterface

//go:generate moq -pkg mocks -out ./mocks/script_verifier_mock.go ../../internal/api ScriptVerifier

//go:generate moq -pkg mocks -out ./mocks/default_server_health_mock.go ../../internal/api ArcDefaultHandlerHealth

//go:generate moq -pkg mocks -out ./mocks/chaintracker_mock.go ../../internal/validator/beef ChainTracker
