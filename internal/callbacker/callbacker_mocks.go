package callbacker

//go:generate moq -pkg mocks -out ./mocks/sender_mock.go ./ SenderI

//go:generate moq -pkg mocks -out ./mocks/callbacker_api_client_mock.go ./callbacker_api/ CallbackerAPIClient

//go:generate moq -pkg mocks -out ./mocks/processor_store_mock.go ./store/ ProcessorStore

//go:generate moq -pkg mocks -out ./mocks/health_watch_server_mock.go ./ HealthWatchServer
