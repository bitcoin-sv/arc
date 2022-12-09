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

	"github.com/TAAL-GmbH/arc"
	"github.com/TAAL-GmbH/arc/config"
	"github.com/TAAL-GmbH/arc/models"
	"github.com/TAAL-GmbH/arc/test"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var contentTypes = []string{
	echo.MIMETextPlain,
	echo.MIMEApplicationJSON,
	echo.MIMEOctetStream,
}

const (
	testMinerKey = "KznvCNc6Yf4iztSThoMH6oHWzH9EgjfodKxmeuUGPq5DEX5maspS"
	testMinerID  = "02798913bc057b344de675dac34faafe3dc2f312c758cd9068209f810877306d66"
	// TODO same as validator, store somewhere centrally?
	validTx         = "0100000001c04e2d8baa442e7445f81ec12c68eb841027211a199176e653fb5e7ef78da0ec8f0000006a4730440220068baf45d1adceba1319511cae65f3c90ec9cdca6ca56350aa6e8e3488424790022033f5efb1740d04f9836c20c07a58ef726d561a51aafc7d3d852bacae2bc30ebb4121035a92fd2b399f6b34941ed245ceb25a416fd2e1af815b315b744293c81fe532c3ffffffff01000000000000000044006a20506b2b9386e05d31a7122b20dec0646b48f690c2e682d3ff3b171cbaf03df11720f360068507c5c9a92ccc74c832665cfa2de09d14f1fc5264193f236e2eec5e4600000000"
	validExtendedTx = "010000000000000000ef01eca08df77e5efb53e67691191a21271084eb682cc11ef845742e44aa8b2d4ec08f0000006a4730440220068baf45d1adceba1319511cae65f3c90ec9cdca6ca56350aa6e8e3488424790022033f5efb1740d04f9836c20c07a58ef726d561a51aafc7d3d852bacae2bc30ebb4121035a92fd2b399f6b34941ed245ceb25a416fd2e1af815b315b744293c81fe532c3ffffffff0c000000000000001976a914cbf0f02390a1de60a00d47ad076fab8378d95c6a88ac01000000000000000044006a20506b2b9386e05d31a7122b20dec0646b48f690c2e682d3ff3b171cbaf03df11720f360068507c5c9a92ccc74c832665cfa2de09d14f1fc5264193f236e2eec5e4600000000"
	validTxID       = "9ba52edfdf32edac89db53b3a384ae26d65c21c2c70189692a8faa5cbb2dee33"
)

func TestNewDefault(t *testing.T) {
	t.Run("simple init", func(t *testing.T) {
		testClient := &test.Client{}
		defaultHandler, err := NewDefault(testClient)
		require.NoError(t, err)
		assert.NotNil(t, defaultHandler)
	})
}

func TestGetArcV1Fees(t *testing.T) { //nolint:funlen
	t.Run("no policy", func(t *testing.T) {
		testStore := &test.Datastore{}
		testClient := &test.Client{
			Store: testStore,
			Node:  nil,
		}

		defaultHandler, err := NewDefault(testClient)
		require.NoError(t, err)
		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/arc/v1/fees", strings.NewReader(""))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		ctx := e.NewContext(req, rec)

		err = defaultHandler.GetArcV1Fees(ctx)
		require.Nil(t, err)
		require.Equal(t, int(arc.ErrStatusNotFound), rec.Code)

		response := rec.Body.Bytes()
		var mError arc.ErrorFields
		_ = json.Unmarshal(response, &mError)
		assert.Equal(t, arc.ErrStatusNotFound, arc.ErrorCode(mError.Status))
	})

	t.Run("default fees", func(t *testing.T) {
		testStore := &test.Datastore{}
		addFees(testStore)
		testClient := &test.Client{
			Store: testStore,
			Node:  nil, // not needed here
		}

		defaultHandler, err := NewDefault(testClient)
		require.NoError(t, err)
		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/arc/v1/fees", strings.NewReader(""))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		ctx := e.NewContext(req, rec)

		err = defaultHandler.GetArcV1Fees(ctx)
		require.Nil(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		bPolicy := rec.Body.Bytes()
		var feesResponse arc.FeesResponse
		_ = json.Unmarshal(bPolicy, &feesResponse)

		require.NotNil(t, feesResponse.Fees)
		fees := *feesResponse.Fees
		assert.Equal(t, arc.Standard, fees[0].FeeType)
		assert.Equal(t, uint64(123), fees[0].MiningFee.Bytes)
		assert.Equal(t, uint64(12), fees[0].MiningFee.Satoshis)
		assert.Equal(t, uint64(321), fees[0].RelayFee.Bytes)
		assert.Equal(t, uint64(32), fees[0].RelayFee.Satoshis)
	})
}

func TestPostArcV1Tx(t *testing.T) { //nolint:funlen
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
			req := httptest.NewRequest(http.MethodPost, "/arc/v1/tx", strings.NewReader(""))
			req.Header.Set(echo.HeaderContentType, contentType)
			rec := httptest.NewRecorder()
			ctx := e.NewContext(req, rec)

			err = defaultHandler.PostArcV1Tx(ctx, arc.PostArcV1TxParams{})
			require.NoError(t, err)
			assert.Equal(t, rec.Code, arc.ErrMalformed.Status)
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
		req := httptest.NewRequest(http.MethodPost, "/arc/v1/tx", strings.NewReader(""))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationXML)
		rec := httptest.NewRecorder()
		ctx := e.NewContext(req, rec)

		err = defaultHandler.PostArcV1Tx(ctx, arc.PostArcV1TxParams{})
		require.NoError(t, err)
		assert.Equal(t, rec.Code, arc.ErrBadRequest.Status)
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
			echo.MIMEOctetStream:     "could not read varint type: EOF",
		}

		for contentType, expectedError := range expectedErrors {
			rec, ctx := createEchoRequest(strings.NewReader("test"), contentType, "/arc/v1/tx")
			err = defaultHandler.PostArcV1Tx(ctx, arc.PostArcV1TxParams{})
			require.NoError(t, err)
			assert.Equal(t, arc.ErrMalformed.Status, rec.Code)

			b := rec.Body.Bytes()
			var bErr arc.ErrorMalformed
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
			addFees(testStore)
			rec, ctx := createEchoRequest(inputTx, contentType, "/arc/v1/tx")
			err = defaultHandler.PostArcV1Tx(ctx, arc.PostArcV1TxParams{})
			require.NoError(t, err)
			assert.Equal(t, arc.ErrStatusTxFormat, arc.ErrorCode(rec.Code))

			b := rec.Body.Bytes()
			var bErr arc.ErrorFee
			_ = json.Unmarshal(b, &bErr)

			assert.Equal(t, "arc error 460: transaction is not in extended format", *bErr.ExtraInfo)
		}
	})

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

		validExtendedTxBytes, _ := hex.DecodeString(validExtendedTx)
		inputTxs := map[string]io.Reader{
			echo.MIMETextPlain:       strings.NewReader(validExtendedTx),
			echo.MIMEApplicationJSON: strings.NewReader("\"" + validExtendedTx + "\""),
			echo.MIMEOctetStream:     bytes.NewReader(validExtendedTxBytes),
		}

		for contentType, inputTx := range inputTxs {
			addFees(testStore)
			rec, ctx := createEchoRequest(inputTx, contentType, "/arc/v1/tx")
			err = defaultHandler.PostArcV1Tx(ctx, arc.PostArcV1TxParams{})
			require.NoError(t, err)
			assert.Equal(t, http.StatusCreated, rec.Code)

			b := rec.Body.Bytes()
			var bResponse arc.TransactionResponse
			_ = json.Unmarshal(b, &bResponse)

			require.Equal(t, validTxID, bResponse.Txid)
		}
	})
}

func addFees(testStore *test.Datastore) {
	testStore.GetModelResult = []interface{}{
		// this mocks the database record being returned
		&models.Fees{
			{
				FeeType: arc.Standard,
				MiningFee: arc.FeeAmount{
					Satoshis: 50,
					Bytes:    1000,
				},
				RelayFee: arc.FeeAmount{
					Satoshis: 50,
					Bytes:    1000,
				},
			}, {
				FeeType: arc.Data,
				MiningFee: arc.FeeAmount{
					Satoshis: 50,
					Bytes:    1000,
				},
				RelayFee: arc.FeeAmount{
					Satoshis: 50,
					Bytes:    1000,
				},
			},
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
