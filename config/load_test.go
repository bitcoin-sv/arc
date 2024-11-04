package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Load(t *testing.T) {
	t.Run("default load", func(t *testing.T) {
		// given
		expectedConfig := getDefaultArcConfig()

		// when
		actualConfig, err := Load()
		require.NoError(t, err, "error loading config")

		// then
		assert.Equal(t, expectedConfig, actualConfig)
	})

	t.Run("partial file override", func(t *testing.T) {
		// given
		expectedConfig := getDefaultArcConfig()

		// when
		actualConfig, err := Load("./test_files/")
		require.NoError(t, err, "error loading config")

		// then
		// verify not overriden default example value
		assert.Equal(t, expectedConfig.GrpcMessageSize, actualConfig.GrpcMessageSize)

		// verify correct override
		assert.Equal(t, "INFO", actualConfig.LogLevel)
		assert.Equal(t, "text", actualConfig.LogFormat)
		assert.Equal(t, "mainnet", actualConfig.Network)
		assert.Equal(t, 18335, actualConfig.Broadcasting.Unicast.Peers[2].Port.P2P)
		assert.Equal(t, "172.28.56.77", actualConfig.Broadcasting.Multicast.MulticastGroups[0])
		assert.Equal(t, true, actualConfig.Broadcasting.Multicast.Ipv6Enabled)
		assert.Equal(t, "unicast", actualConfig.Broadcasting.Mode)
		assert.Equal(t, "eth1", *actualConfig.Broadcasting.Multicast.Interfaces[1])
		assert.NotNil(t, actualConfig.Tracing)
		assert.Equal(t, "http://tracing:1234", actualConfig.Tracing.DialAddr)
	})
}
