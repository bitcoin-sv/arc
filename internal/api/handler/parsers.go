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
	body, err := io.ReadAll(request.Body)
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
		txHex = body
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

	contentType := request.Header.Get(echo.HeaderContentType)

	switch contentType {
	case echo.MIMETextPlain:
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
	case echo.MIMEApplicationJSON:
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
	case echo.MIMEOctetStream:
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		txHex = body
	default:
		return nil, fmt.Errorf("given content-type %s does not match any of the allowed content-types", contentType)
	}

	if len(txHex) == 0 {
		return nil, ErrEmptyBody
	}

	return txHex, nil
}
