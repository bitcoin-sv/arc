package config

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestGetPeerSettings(t *testing.T) {
	t.Run("get peer settings from config", func(t *testing.T) {
		expectedPeerSettings := []Peer{
			{
				Host: "localhost",
				Port: PeerPort{P2P: 18333, ZMQ: 28333},
			},
			{
				Host: "localhost",
				Port: PeerPort{P2P: 18334, ZMQ: 28334},
			},
			{
				Host: "localhost",
				Port: PeerPort{P2P: 18335, ZMQ: 28335},
			},
		}

		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath("./testdata")
		err := viper.ReadInConfig()
		require.NoError(t, err)

		peerSettings, err := GetPeerSettings()
		require.NoError(t, err)

		require.Equal(t, expectedPeerSettings, peerSettings)
	})
}
