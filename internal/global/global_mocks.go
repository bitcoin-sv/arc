package global

//go:generate moq -pkg mocks -out ./mocks/transaction_handler_mock.go . TransactionHandler

//go:generate moq -pkg mocks -out ./mocks/blocktx_client_mock.go . BlocktxClient

//go:generate moq -pkg mocks -out ./mocks/stoppable_mock.go . Stoppable

//go:generate moq -pkg mocks -out ./mocks/stoppable_with_error_mock.go . StoppableWithError

//go:generate moq -pkg mocks -out ./mocks/stoppable_with_context_mock.go . StoppableWithContext
