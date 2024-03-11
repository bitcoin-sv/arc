package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestGetDefaultPolicy(t *testing.T) {
	t.Run("get default policy from config", func(t *testing.T) {
		viper.SetConfigName("config/config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath("./testdata")
		err := viper.ReadInConfig()
		require.NoError(t, err)

		policy, err := GetDefaultPolicy()
		require.NoError(t, err)

		require.Equal(t, defaultPolicy, policy)
	})
}

func TestCheckSwagger(t *testing.T) {
	tt := []struct {
		name string
		path string
	}{
		{
			name: "valid request",
			path: "/v1/policy",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(""))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

			CheckSwagger(e)
		})
	}
}
