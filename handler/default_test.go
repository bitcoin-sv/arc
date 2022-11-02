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

var postTypes = []string{
	echo.MIMETextPlain,
	echo.MIMEApplicationJSON,
	echo.MIMEOctetStream,
}

var (
	validTx = "02000000010f117b3f9ea4955d5c592c61838bea10096fc88ac1ad08561a9bcabd715a088200000000494830450221008fd0e0330470ac730b9f6b9baf1791b76859cbc327e2e241f3ebeb96561a719602201e73532eb1312a00833af276d636254b8aa3ecbb445324fb4c481f2a493821fb41feffffff02a0860100000000001976a914b7b88045cc16f442a0c3dcb3dc31ecce8d156e7388ac605c042a010000001976a9147a904b8ae0c2f9d74448993029ad3c040ebdd69a88ac66000000"
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

func TestPostMapiV2Tx(t *testing.T) {
	t.Run("empty tx", func(t *testing.T) {
		testStore := &test.Datastore{}
		testClient := &test.Client{
			Store: testStore,
			Node:  nil,
		}

		defaultHandler, err := NewDefault(testClient)
		require.NoError(t, err)

		for _, postType := range postTypes {
			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/mapi/v2/tx", strings.NewReader(""))
			req.Header.Set(echo.HeaderContentType, postType)
			rec := httptest.NewRecorder()
			ctx := e.NewContext(req, rec)

			err = defaultHandler.PostMapiV2Tx(ctx, mapi.PostMapiV2TxParams{})
			require.NoError(t, err)
			assert.Equal(t, rec.Code, mapi.ErrMalformed.Status)
		}
	})

	t.Run("invalid mime type", func(t *testing.T) {
		testStore := &test.Datastore{}
		testClient := &test.Client{
			Store: testStore,
			Node:  nil,
		}

		defaultHandler, err := NewDefault(testClient)
		require.NoError(t, err)

		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/mapi/v2/tx", strings.NewReader(""))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationXML)
		rec := httptest.NewRecorder()
		ctx := e.NewContext(req, rec)

		err = defaultHandler.PostMapiV2Tx(ctx, mapi.PostMapiV2TxParams{})
		require.NoError(t, err)
		assert.Equal(t, rec.Code, mapi.ErrBadRequest.Status)

	})

	t.Run("invalid tx", func(t *testing.T) {
		testStore := &test.Datastore{}
		testClient := &test.Client{
			Store: testStore,
			Node:  nil,
		}

		defaultHandler, err := NewDefault(testClient)
		require.NoError(t, err)

		expectedErrors := []string{
			"encoding/hex: invalid byte: U+0074 't'",
			"invalid character 'e' in literal true (expecting 'r')",
			"too short to be a tx - even an empty tx has 10 bytes",
		}

		for index, postType := range postTypes {
			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/mapi/v2/tx", strings.NewReader("test"))
			req.Header.Set(echo.HeaderContentType, postType)
			rec := httptest.NewRecorder()
			ctx := e.NewContext(req, rec)

			err = defaultHandler.PostMapiV2Tx(ctx, mapi.PostMapiV2TxParams{})
			require.NoError(t, err)
			assert.Equal(t, mapi.ErrMalformed.Status, rec.Code)

			b := rec.Body.Bytes()
			var bErr mapi.ErrorMalformed
			_ = json.Unmarshal(b, &bErr)

			require.NotNil(t, bErr.ExtraInfo)
			assert.Equal(t, expectedErrors[index], *bErr.ExtraInfo)
		}
	})

	t.Run("valid tx - text/plain", func(t *testing.T) {
		testStore := &test.Datastore{}
		testNode := &test.Node{}
		testClient := &test.Client{
			Store: testStore,
			Node:  testNode,
		}

		defaultHandler, err := NewDefault(testClient)
		require.NoError(t, err)

		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/mapi/v2/tx", strings.NewReader(validTx))
		req.Header.Set(echo.HeaderContentType, echo.MIMETextPlain)
		rec := httptest.NewRecorder()
		ctx := e.NewContext(req, rec)

		err = defaultHandler.PostMapiV2Tx(ctx, mapi.PostMapiV2TxParams{})
		require.NoError(t, err)
		assert.Equal(t, mapi.ErrInputs.Status, rec.Code)

		b := rec.Body.Bytes()
		var bErr mapi.ErrorInputs
		_ = json.Unmarshal(b, &bErr)

		require.Nil(t, bErr.ExtraInfo)
	})
}
