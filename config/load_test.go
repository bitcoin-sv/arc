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
		actualConfig, err := Load("./test_files/config.yaml")
		require.NoError(t, err, "error loading config")

		// then
		// verify not overridden default example value
		assert.Equal(t, expectedConfig.Common.GrpcMessageSize, actualConfig.Common.GrpcMessageSize)

		// verify correct override
		assert.Equal(t, "INFO", actualConfig.Common.LogLevel)
		assert.Equal(t, "text", actualConfig.Common.LogFormat)
		assert.Equal(t, "mainnet", actualConfig.Common.Network)
		assert.NotNil(t, actualConfig.Common.Tracing)
		assert.Equal(t, "http://tracing:1234", actualConfig.Common.Tracing.DialAddr)
	})
}
