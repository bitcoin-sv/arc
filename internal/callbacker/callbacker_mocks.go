package callbacker

// from callbacker.go
//go:generate moq -out ./callbacker_mock.go ./ SenderI

//go:generate moq -out ./callbacker_api/callbacker_api_client_mock.go ./callbacker_api/ CallbackerAPIClient
