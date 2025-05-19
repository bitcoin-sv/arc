package api

//go:generate moq -pkg mocks -out ./mocks/client_interface_mock.go ../../pkg/api ClientInterface

//go:generate moq -pkg mocks -out ./mocks/script_verifier_mock.go ../../internal/api ScriptVerifier
