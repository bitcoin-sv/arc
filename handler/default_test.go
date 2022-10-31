package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/TAAL-GmbH/mapi/client"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDefault(t *testing.T) {
	t.Run("simple init", func(t *testing.T) {
		testClient := &client.TestClient{}
		defaultHandler, err := NewDefault(testClient)
		require.NoError(t, err)
		assert.NotNil(t, defaultHandler)
	})
}

func TestGetMapiV2Policy(t *testing.T) {
	t.Run("no policy", func(t *testing.T) {
		testClient := &client.TestClient{
			Store: nil,
			Node:  nil,
		}
		defaultHandler, err := NewDefault(testClient)
		require.NoError(t, err)
		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		ctx := e.NewContext(req, rec)

		assert.NotNil(t, defaultHandler.GetMapiV2Policy(ctx))
	})
}
