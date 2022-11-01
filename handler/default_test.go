package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/TAAL-GmbH/mapi"
	"github.com/TAAL-GmbH/mapi/models"
	"github.com/TAAL-GmbH/mapi/test"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDefault(t *testing.T) {
	t.Run("simple init", func(t *testing.T) {
		testClient := &test.Client{}
		defaultHandler, err := NewDefault(testClient)
		require.NoError(t, err)
		assert.NotNil(t, defaultHandler)
	})
}

func TestGetMapiV2Policy(t *testing.T) {
	t.Run("no policy", func(t *testing.T) {
		testStore := &test.Datastore{}
		testClient := &test.Client{
			Store: testStore,
			Node:  nil,
		}

		defaultHandler, err := NewDefault(testClient)
		require.NoError(t, err)
		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/mapi/v2/policy", strings.NewReader(""))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		ctx := e.NewContext(req, rec)

		err = defaultHandler.GetMapiV2Policy(ctx)
		require.NotNil(t, err)
		require.IsType(t, &echo.HTTPError{}, err) // echo doesn't properly implement error ?
		eErr := err.(*echo.HTTPError)
		assert.Equal(t, http.StatusNotFound, eErr.Code)
		assert.Equal(t, http.StatusText(http.StatusNotFound), eErr.Message)
	})

	t.Run("default policy", func(t *testing.T) {
		testStore := &test.Datastore{}
		testStore.GetModelResult = []interface{}{
			// this mocks the database record being returned
			&models.Policy{
				ID: "test-id",
				Fees: models.Fees{{
					FeeType: mapi.Standard,
					MiningFee: mapi.FeeAmount{
						Bytes:    123,
						Satoshis: 12,
					},
					RelayFee: mapi.FeeAmount{
						Bytes:    321,
						Satoshis: 32,
					},
				}},
				Policies: models.Policies{
					"policy1-key": "policy1-value",
				},
				ClientID: "test-client",
			},
		}
		testClient := &test.Client{
			Store: testStore,
			Node:  nil, // not needed here
		}

		defaultHandler, err := NewDefault(testClient)
		require.NoError(t, err)
		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/mapi/v2/policy", strings.NewReader(""))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		ctx := e.NewContext(req, rec)

		err = defaultHandler.GetMapiV2Policy(ctx)
		require.Nil(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		bPolicy := rec.Body.Bytes()
		var policy mapi.Policy
		_ = json.Unmarshal(bPolicy, &policy)

		assert.Equal(t, mapi.APIVersion, policy.ApiVersion)

		require.NotNil(t, policy.Fees)
		fees := *policy.Fees
		assert.Equal(t, mapi.Standard, fees[0].FeeType)
		assert.Equal(t, uint64(123), fees[0].MiningFee.Bytes)
		assert.Equal(t, uint64(12), fees[0].MiningFee.Satoshis)
		assert.Equal(t, uint64(321), fees[0].RelayFee.Bytes)
		assert.Equal(t, uint64(32), fees[0].RelayFee.Satoshis)

		require.NotNil(t, policy.Policies)
		policies := *policy.Policies
		assert.Equal(t, "policy1-value", policies["policy1-key"])
	})
}
