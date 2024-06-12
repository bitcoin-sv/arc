package config

func getDefaultArcConfig() *ArcConfig {
	return &ArcConfig{
		PeerRpc: getDefaultPeerRpcConfig(),
	}
}

func getDefaultPeerRpcConfig() *PeerRpcConfig {
	return &PeerRpcConfig{
		Password: "bitcoin",
		User:     "bitcoin",
		Host:     "localhost",
		Port:     18332,
	}
}
