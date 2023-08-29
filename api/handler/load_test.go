package handler

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestGetDefaultPolicy(t *testing.T) {
	t.Run("get default policy from config", func(t *testing.T) {

		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath("./testdata")
		err := viper.ReadInConfig()
		require.NoError(t, err)

		policy, err := GetDefaultPolicy()
		require.NoError(t, err)

		require.Equal(t, defaultPolicy, policy)
	})
}
