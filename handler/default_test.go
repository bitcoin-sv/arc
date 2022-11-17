package handler

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io"
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

var contentTypes = []string{
	echo.MIMETextPlain,
	echo.MIMEApplicationJSON,
	echo.MIMEOctetStream,
}

var (
	testMinerKey = "KznvCNc6Yf4iztSThoMH6oHWzH9EgjfodKxmeuUGPq5DEX5maspS"
	testMinerID  = "02798913bc057b344de675dac34faafe3dc2f312c758cd9068209f810877306d66"
	// TODO same as validator, store somewhere centrally?
	validTx   = "02000000010f117b3f9ea4955d5c592c61838bea10096fc88ac1ad08561a9bcabd715a088200000000494830450221008fd0e0330470ac730b9f6b9baf1791b76859cbc327e2e241f3ebeb96561a719602201e73532eb1312a00833af276d636254b8aa3ecbb445324fb4c481f2a493821fb41feffffff02a0860100000000001976a914b7b88045cc16f442a0c3dcb3dc31ecce8d156e7388ac605c042a010000001976a9147a904b8ae0c2f9d74448993029ad3c040ebdd69a88ac66000000"
	validTxID = "daedb57812f407ff91f874b22fc49b6be7b1dd5adc609e6d306bd3c4934f53b0"
	parentTx  = "02000000010000000000000000000000000000000000000000000000000000000000000000ffffffff03520101ffffffff0100f2052a01000000232103b12bda06e5a3e439690bf3996f1d4b81289f4747068a5cbb12786df83ae14c18ac00000000"
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
		require.Nil(t, err)
		require.Equal(t, int(mapi.ErrStatusNotFound), rec.Code)

		response := rec.Body.Bytes()
		var mError mapi.ErrorFields
		_ = json.Unmarshal(response, &mError)
		assert.Equal(t, mapi.ErrStatusNotFound, mapi.ErrStatus(mError.Status))
	})

	t.Run("default policy", func(t *testing.T) {
		testStore := &test.Datastore{}
		addPolicy(testStore)
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

		for _, contentType := range contentTypes {
			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/mapi/v2/tx", strings.NewReader(""))
			req.Header.Set(echo.HeaderContentType, contentType)
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

		expectedErrors := map[string]string{
			echo.MIMETextPlain:       "encoding/hex: invalid byte: U+0074 't'",
			echo.MIMEApplicationJSON: "invalid character 'e' in literal true (expecting 'r')",
			echo.MIMEOctetStream:     "too short to be a tx - even an empty tx has 10 bytes",
		}

		for contentType, expectedError := range expectedErrors {
			rec, ctx := createEchoRequest(strings.NewReader("test"), contentType, "/mapi/v2/tx")
			err = defaultHandler.PostMapiV2Tx(ctx, mapi.PostMapiV2TxParams{})
			require.NoError(t, err)
			assert.Equal(t, mapi.ErrMalformed.Status, rec.Code)

			b := rec.Body.Bytes()
			var bErr mapi.ErrorMalformed
			_ = json.Unmarshal(b, &bErr)

			require.NotNil(t, bErr.ExtraInfo)
			assert.Equal(t, expectedError, *bErr.ExtraInfo)
		}
	})

	t.Run("valid tx - missing inputs", func(t *testing.T) {
		testStore := &test.Datastore{}
		testNode := &test.Node{}
		testClient := &test.Client{
			Store: testStore,
			Node:  testNode,
		}

		defaultHandler, err := NewDefault(testClient)
		require.NoError(t, err)

		validTxBytes, _ := hex.DecodeString(validTx)
		inputTxs := map[string]io.Reader{
			echo.MIMETextPlain:       strings.NewReader(validTx),
			echo.MIMEApplicationJSON: strings.NewReader("\"" + validTx + "\""),
			echo.MIMEOctetStream:     bytes.NewReader(validTxBytes),
		}

		for contentType, inputTx := range inputTxs {
			addPolicy(testStore)
			rec, ctx := createEchoRequest(inputTx, contentType, "/mapi/v2/tx")
			err = defaultHandler.PostMapiV2Tx(ctx, mapi.PostMapiV2TxParams{})
			require.NoError(t, err)
			assert.Equal(t, mapi.ErrStatusFees, mapi.ErrStatus(rec.Code))

			b := rec.Body.Bytes()
			var bErr mapi.ErrorFee
			_ = json.Unmarshal(b, &bErr)

			assert.Equal(t, "mapi error 464: transaction fee is too low", *bErr.ExtraInfo)
		}
	})

	/* TODO this test needs an extended format tx
	t.Run("valid tx", func(t *testing.T) {
		testStore := &test.Datastore{}
		testNode := &test.Node{}
		testClient := &test.Client{
			Store:         testStore,
			Node:          testNode,
			MinerIDConfig: &config.MinerIDConfig{PrivateKey: testMinerKey},
		}

		defaultHandler, err := NewDefault(testClient)
		require.NoError(t, err)

		validTxBytes, _ := hex.DecodeString(validTx)
		inputTxs := map[string]io.Reader{
			echo.MIMETextPlain:       strings.NewReader(validTx),
			echo.MIMEApplicationJSON: strings.NewReader("\"" + validTx + "\""),
			echo.MIMEOctetStream:     bytes.NewReader(validTxBytes),
		}

		for contentType, inputTx := range inputTxs {
			addPolicy(testStore)
			rec, ctx := createEchoRequest(inputTx, contentType, "/mapi/v2/tx")
			err = defaultHandler.PostMapiV2Tx(ctx, mapi.PostMapiV2TxParams{})
			require.NoError(t, err)
			assert.Equal(t, http.StatusCreated, rec.Code)

			b := rec.Body.Bytes()
			var bResponse mapi.TransactionResponse
			_ = json.Unmarshal(b, &bResponse)

			require.Equal(t, mapi.APIVersion, bResponse.ApiVersion)
			require.Equal(t, testMinerID, bResponse.MinerId)
			require.Equal(t, validTxID, bResponse.Txid)
		}
	})
	*/
}

func addPolicy(testStore *test.Datastore) {
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
}

func createEchoRequest(inputTx io.Reader, contentType, target string) (*httptest.ResponseRecorder, echo.Context) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, target, inputTx)
	req.Header.Set(echo.HeaderContentType, contentType)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	return rec, ctx
}
