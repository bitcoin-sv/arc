package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Load(t *testing.T) {
	t.Run("default load", func(t *testing.T) {
		defaultConfig := getDefaultArcConfig()

		config, err := Load()
		require.NoError(t, err, "error loading config")

		assert.Equal(t, defaultConfig, config)
	})

	t.Run("partial file override", func(t *testing.T) {
		defaultConfig := getDefaultArcConfig()

		config, err := Load("./test_files/partial_config.yaml")
		require.NoError(t, err, "error loading config")

		// verify not overriden default example value
		assert.Equal(t, defaultConfig.GrpcMessageSize, config.GrpcMessageSize)

		// verify correct override
		assert.Equal(t, "INFO", config.LogLevel)
		assert.Equal(t, "text", config.LogFormat)
		assert.Equal(t, "mainnet", config.Network)
		assert.NotNil(t, config.Tracing)
		assert.Equal(t, "http://tracing:1234", config.Tracing.DialAddr)
	})
}
