package k8swatcher

//go:generate moq -pkg mocks -out ./mocks/k8s_client_client_mock.go . K8sClient

//go:generate moq -pkg mocks -out ./mocks/ticker_mock.go . Ticker
