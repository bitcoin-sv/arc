package callbacker

// from callbacker.go
//go:generate moq -pkg mocks -out ./mocks/callbacker_mock.go ./ SenderI

//go:generate moq -pkg mocks -out ./mocks/callbacker_api_client_mock.go ./callbacker_api/ CallbackerAPIClient

//go:generate moq -pkg mocks -out ./mocks/store_mock.go ./store/ CallbackerStore
