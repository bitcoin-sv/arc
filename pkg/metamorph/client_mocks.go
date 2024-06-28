package metamorph

//go:generate moq -pkg mocks -out ./mocks/metamorph_api_mock.go ./metamorph_api MetaMorphAPIClient

//go:generate moq -pkg mocks -out ./mocks/transaction_handler_mock.go . TransactionHandler

//go:generate moq -pkg mocks -out ./mocks/metamorph_client_mock.go . TransactionMaintainer

//go:generate moq -pkg mocks -out ./mocks/message_queue_client_mock.go . MessageQueueClient
