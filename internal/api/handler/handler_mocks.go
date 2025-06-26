package handler

//go:generate moq -pkg mocks -out ./mocks/default_validator_mock.go ../../validator DefaultValidator

//go:generate moq -pkg mocks -out ./mocks/beef_validator_mock.go ../../validator BeefValidator
