package handler

//go:generate moq -pkg mocks -skip-ensure -out ./mocks/default_validator_mock.go . DefaultValidator

//go:generate moq -pkg mocks -skip-ensure -out ./mocks/beef_validator_mock.go . BeefValidator
