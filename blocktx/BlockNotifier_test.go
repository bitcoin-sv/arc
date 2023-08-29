package blocktx

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestGetPeerSettings(t *testing.T) {
	t.Run("get peer settings from config", func(t *testing.T) {

		peers := []Peer{
			{
				Host:    "localhost",
				PortP2P: 18333,
				PortZMQ: 28333,
			},
			{
				Host:    "localhost",
				PortP2P: 18334,
				PortZMQ: 28334,
			},
			{
				Host:    "localhost",
				PortP2P: 18335,
				PortZMQ: 28335,
			},
		}

		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath("./testdata")
		err := viper.ReadInConfig()
		require.NoError(t, err)

		peerSettings, err := GetPeerSettings()
		require.NoError(t, err)

		require.Equal(t, peers, peerSettings)

	})
}
