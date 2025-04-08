package broadcaster

// from arc_client.go
//go:generate moq -pkg mocks -out ./mocks/arc_client_mock.go . ArcClient

// from broadcaster.go
//go:generate moq -pkg mocks -out ./mocks/utxo_client_mock.go . UtxoClient

// from mutli_utxo_consolidator.go
//go:generate moq -pkg mocks -out ./mocks/consolidator_mock.go . Consolidator

// from multi_rate_broadcaster.go
//go:generate moq -pkg mocks -out ./mocks/rate_broadcaster_mock.go . RateBroadcaster

// from multi_utxo_creator.go
//go:generate moq -pkg mocks -out ./mocks/utxo_creator_mock.go . Creator

// from rate_broadcaster.go
//go:generate moq -pkg mocks -out ./mocks/ticker_mock.go . Ticker
