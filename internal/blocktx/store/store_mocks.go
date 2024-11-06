package store

//go:generate moq -pkg mocks -out ./mocks/blocktx_store_mock.go . BlocktxStore
//go:generate moq -pkg mocks -out ./mocks/blocktx_db_tx_mock.go . DbTransaction
