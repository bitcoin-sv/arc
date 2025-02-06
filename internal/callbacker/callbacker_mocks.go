package callbacker

//go:generate moq -pkg mocks -out ./mocks/sender_mock.go ./ SenderI

//go:generate moq -pkg mocks -out ./mocks/send_manager_mock.go ./ SendManagerI

//go:generate moq -pkg mocks -out ./mocks/callbacker_api_client_mock.go ./callbacker_api/ CallbackerAPIClient

//go:generate moq -pkg mocks -out ./mocks/processor_store_mock.go ./store/ ProcessorStore

//go:generate moq -pkg mocks -out ./mocks/dipatcher_mock.go ./ Dispatcher

//go:generate moq -pkg mocks -out ./mocks/mq_client_mock.go ./ MessageQueueClient

//go:generate moq -pkg mocks -out ./mocks/jetstream_message_mock.go ./ JetstreamMsg
