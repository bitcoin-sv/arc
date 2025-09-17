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
		// verify not overridden default example value
		assert.Equal(t, expectedConfig.Global.GrpcMessageSize, actualConfig.Global.GrpcMessageSize)

		// verify correct override
		assert.Equal(t, "INFO", actualConfig.Global.LogLevel)
		assert.Equal(t, "text", actualConfig.Global.LogFormat)
		assert.Equal(t, "mainnet", actualConfig.Global.Network)
		assert.NotNil(t, actualConfig.Global.Tracing)
		assert.Equal(t, "http://tracing:1234", actualConfig.Global.Tracing.DialAddr)
	})
}
