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
	"time"

	"github.com/TAAL-GmbH/arc/api"
	"github.com/TAAL-GmbH/arc/api/test"
	"github.com/TAAL-GmbH/arc/api/transactionHandler"
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
	validTx         = "0100000001358eb38f1f910e76b33788ff9395a5d2af87721e950ebd3d60cf64bb43e77485010000006a47304402203be8a3ba74e7b770afa2addeff1bbc1eaeb0cedf6b4096c8eb7ec29f1278752602205dc1d1bedf2cab46096bb328463980679d4ce2126cdd6ed191d6224add9910884121021358f252895263cd7a85009fcc615b57393daf6f976662319f7d0c640e6189fcffffffff02bf010000000000001976a91449f066fccf8d392ff6a0a33bc766c9f3436c038a88acfc080000000000001976a914a7dcbd14f83c564e0025a57f79b0b8b591331ae288ac00000000"
	validExtendedTx = "010000000000000000ef01358eb38f1f910e76b33788ff9395a5d2af87721e950ebd3d60cf64bb43e77485010000006a47304402203be8a3ba74e7b770afa2addeff1bbc1eaeb0cedf6b4096c8eb7ec29f1278752602205dc1d1bedf2cab46096bb328463980679d4ce2126cdd6ed191d6224add9910884121021358f252895263cd7a85009fcc615b57393daf6f976662319f7d0c640e6189fcffffffffc70a0000000000001976a914f1e6837cf17b485a1dcea9e943948fafbe5e9f6888ac02bf010000000000001976a91449f066fccf8d392ff6a0a33bc766c9f3436c038a88acfc080000000000001976a914a7dcbd14f83c564e0025a57f79b0b8b591331ae288ac00000000"
	validTxID       = "a147cc3c71cc13b29f18273cf50ffeb59fc9758152e2b33e21a8092f0b049118"
)

func TestNewDefault(t *testing.T) {
	t.Run("simple init", func(t *testing.T) {
		defaultHandler, err := NewDefault(nil)
		require.NoError(t, err)
		assert.NotNil(t, defaultHandler)
	})
}

func TestGetArcV1Fees(t *testing.T) { //nolint:funlen
	t.Run("default fees", func(t *testing.T) {
		defaultHandler, err := NewDefault(nil)
		require.NoError(t, err)
		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/v1/fees", strings.NewReader(""))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		ctx := e.NewContext(req, rec)

		err = defaultHandler.GETFees(ctx)
		require.Nil(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		bPolicy := rec.Body.Bytes()
		var feesResponse api.FeesResponse
		_ = json.Unmarshal(bPolicy, &feesResponse)

		require.NotNil(t, feesResponse.Fees)
		fees := *feesResponse.Fees
		assert.Equal(t, api.Data, fees[0].FeeType)
		assert.Equal(t, uint64(5), fees[0].MiningFee.Satoshis)
		assert.Equal(t, uint64(1000), fees[0].MiningFee.Bytes)
		assert.Equal(t, uint64(5), fees[0].RelayFee.Satoshis)
		assert.Equal(t, uint64(1000), fees[0].RelayFee.Bytes)
	})
}

func TestPostArcV1Tx(t *testing.T) { //nolint:funlen
	t.Run("empty tx", func(t *testing.T) {
		defaultHandler, err := NewDefault(nil)
		require.NoError(t, err)

		for _, contentType := range contentTypes {
			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/v1/tx", strings.NewReader(""))
			req.Header.Set(echo.HeaderContentType, contentType)
			rec := httptest.NewRecorder()
			ctx := e.NewContext(req, rec)

			err = defaultHandler.POSTTransaction(ctx, api.POSTTransactionParams{})
			require.NoError(t, err)
			assert.Equal(t, rec.Code, api.ErrMalformed.Status)
		}
	})

	t.Run("invalid mime type", func(t *testing.T) {
		defaultHandler, err := NewDefault(nil)
		require.NoError(t, err)

		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/v1/tx", strings.NewReader(""))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationXML)
		rec := httptest.NewRecorder()
		ctx := e.NewContext(req, rec)

		err = defaultHandler.POSTTransaction(ctx, api.POSTTransactionParams{})
		require.NoError(t, err)
		assert.Equal(t, rec.Code, api.ErrBadRequest.Status)
	})

	t.Run("invalid tx", func(t *testing.T) {
		defaultHandler, err := NewDefault(nil)
		require.NoError(t, err)

		expectedErrors := map[string]string{
			echo.MIMETextPlain:       "encoding/hex: invalid byte: U+0074 't'",
			echo.MIMEApplicationJSON: "invalid character 'e' in literal true (expecting 'r')",
			echo.MIMEOctetStream:     "could not read varint type: EOF",
		}

		for contentType, expectedError := range expectedErrors {
			rec, ctx := createEchoRequest(strings.NewReader("test"), contentType, "/v1/tx")
			err = defaultHandler.POSTTransaction(ctx, api.POSTTransactionParams{})
			require.NoError(t, err)
			assert.Equal(t, api.ErrMalformed.Status, rec.Code)

			b := rec.Body.Bytes()
			var bErr api.ErrorMalformed
			_ = json.Unmarshal(b, &bErr)

			require.NotNil(t, bErr.ExtraInfo)
			assert.Equal(t, expectedError, *bErr.ExtraInfo)
		}
	})

	t.Run("valid tx - missing inputs", func(t *testing.T) {
		testNode := &test.Node{}
		defaultHandler, err := NewDefault(testNode)
		require.NoError(t, err)

		validTxBytes, _ := hex.DecodeString(validTx)
		inputTxs := map[string]io.Reader{
			echo.MIMETextPlain:       strings.NewReader(validTx),
			echo.MIMEApplicationJSON: strings.NewReader("{\"rawTx\":\"" + validTx + "\"}"),
			echo.MIMEOctetStream:     bytes.NewReader(validTxBytes),
		}

		for contentType, inputTx := range inputTxs {
			rec, ctx := createEchoRequest(inputTx, contentType, "/v1/tx")
			err = defaultHandler.POSTTransaction(ctx, api.POSTTransactionParams{})
			require.NoError(t, err)
			assert.Equal(t, api.ErrStatusTxFormat, api.StatusCode(rec.Code))

			b := rec.Body.Bytes()
			var bErr api.ErrorFee
			_ = json.Unmarshal(b, &bErr)

			assert.Equal(t, "parent transaction not found", *bErr.ExtraInfo)
		}
	})

	t.Run("valid tx", func(t *testing.T) {
		testNode := &test.Node{}
		txResult := &transactionHandler.TransactionStatus{
			TxID:        validTxID,
			BlockHash:   "",
			BlockHeight: 0,
			Status:      "OK",
			Timestamp:   time.Now().Unix(),
		}
		// set the node/metamorph responses for the 3 test requests
		testNode.SubmitTransactionResult = append(testNode.SubmitTransactionResult, txResult, txResult, txResult)

		defaultHandler, err := NewDefault(testNode)
		require.NoError(t, err)

		validExtendedTxBytes, _ := hex.DecodeString(validExtendedTx)
		inputTxs := map[string]io.Reader{
			echo.MIMETextPlain:       strings.NewReader(validExtendedTx),
			echo.MIMEApplicationJSON: strings.NewReader("{\"rawTx\":\"" + validExtendedTx + "\"}"),
			echo.MIMEOctetStream:     bytes.NewReader(validExtendedTxBytes),
		}

		for contentType, inputTx := range inputTxs {
			rec, ctx := createEchoRequest(inputTx, contentType, "/v1/tx")
			err = defaultHandler.POSTTransaction(ctx, api.POSTTransactionParams{})
			require.NoError(t, err)
			assert.Equal(t, http.StatusCreated, rec.Code)

			b := rec.Body.Bytes()
			var bResponse api.TransactionResponse
			_ = json.Unmarshal(b, &bResponse)

			require.Equal(t, validTxID, *bResponse.Txid)
		}
	})
}

func createEchoRequest(inputTx io.Reader, contentType, target string) (*httptest.ResponseRecorder, echo.Context) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, target, inputTx)
	req.Header.Set(echo.HeaderContentType, contentType)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	return rec, ctx
}
