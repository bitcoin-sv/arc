package broadcaster

// from arc_client.go
//go:generate moq -pkg mocks -out ./mocks/arc_client_mock.go . ArcClient

// from broadcaster.go
//go:generate moq -pkg mocks -out ./mocks/utxo_client_mock.go . UtxoClient

// from mutli_utxo_consolidator.go
//go:generate moq -pkg mocks -out ./mocks/consolidator_mock.go . Consolidator
