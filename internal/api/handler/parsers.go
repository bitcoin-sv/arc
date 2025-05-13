package handler

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/bitcoin-sv/arc/pkg/api"
	"github.com/labstack/echo/v4"
)

var ErrEmptyBody = errors.New("no transaction found - empty request body")

func parseTransactionFromRequest(request *http.Request) ([]byte, error) {
	body, err := getBodyFromRequest(request)
	if err != nil {
		return nil, err
	}

	if len(body) == 0 {
		return nil, ErrEmptyBody
	}

	var txHex []byte

	contentType := request.Header.Get(echo.HeaderContentType)

	switch contentType {
	case echo.MIMETextPlain:
		txHex, err = hex.DecodeString(string(body))
		if err != nil {
			return nil, err
		}
	case echo.MIMEApplicationJSON:
		var txBody api.POSTTransactionJSONRequestBody
		if err = json.Unmarshal(body, &txBody); err != nil {
			return nil, err
		}

		txHex, err = hex.DecodeString(txBody.RawTx)
		if err != nil {
			return nil, err
		}
	case echo.MIMEOctetStream:
		return body, nil
	default:
		return nil, fmt.Errorf("given content-type %s does not match any of the allowed content-types", contentType)
	}

	if len(txHex) == 0 {
		return nil, ErrEmptyBody
	}

	return txHex, nil
}

func parseTransactionsFromRequest(request *http.Request) ([]byte, error) {
	var txHex []byte
	var err error

	contentType := request.Header.Get(echo.HeaderContentType)

	switch contentType {
	case echo.MIMETextPlain:
		if txHex, err = getTxHexFromMIMETextPlain(request); err != nil {
			return nil, err
		}
	case echo.MIMEApplicationJSON:
		if txHex, err = getTxHexFromMIMEApplicationJSON(request); err != nil {
			return nil, err
		}
	case echo.MIMEOctetStream:
		if txHex, err = getTxHexFromMIMEOctetStream(request); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("given content-type %s does not match any of the allowed content-types", contentType)
	}

	if len(txHex) == 0 {
		return nil, ErrEmptyBody
	}

	return txHex, nil
}

func getTxHexFromMIMEApplicationJSON(request *http.Request) ([]byte, error) {
	var txHex []byte
	var txBody api.POSTTransactionsJSONRequestBody
	if err := json.NewDecoder(request.Body).Decode(&txBody); err != nil {
		return nil, err
	}
	for _, tx := range txBody {
		partialHex, err := hex.DecodeString(tx.RawTx)
		if err != nil {
			return nil, err
		}
		txHex = append(txHex, partialHex...)
	}
	return txHex, nil
}

func getTxHexFromMIMETextPlain(request *http.Request) ([]byte, error) {
	var txHex []byte
	scanner := bufio.NewScanner(request.Body)

	for scanner.Scan() {
		partialHex, err := hex.DecodeString(scanner.Text())
		if err != nil {
			return nil, err
		}
		txHex = append(txHex, partialHex...)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return txHex, nil
}

func getTxHexFromMIMEOctetStream(request *http.Request) ([]byte, error) {
	return getBodyFromRequest(request)
}

func getBodyFromRequest(request *http.Request) ([]byte, error) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}

	if len(body) == 0 {
		return nil, ErrEmptyBody
	}
	return body, nil
}
