package global

//go:generate moq -pkg mocks -out ./mocks/transaction_handler_mock.go . TransactionHandler

//go:generate moq -pkg mocks -out ./mocks/blocktx_client_mock.go . BlocktxClient
