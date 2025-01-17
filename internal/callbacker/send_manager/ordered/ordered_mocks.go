package ordered

//go:generate moq -pkg mocks -out ./mocks/store_mock.go . SendManagerStore

//go:generate moq -pkg mocks -out ./mocks/sender_mock.go . Sender
