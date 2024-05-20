package handler

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bitcoin-sv/arc/pkg/api"
)

func parseTransactionFromRequest(request *http.Request) ([]byte, *api.ErrorFields) {
	requestBody := request.Body
	contentType := request.Header.Get("Content-Type")

	body, err := io.ReadAll(requestBody)
	if err != nil {
		return nil, api.NewErrorFields(api.ErrStatusBadRequest, err.Error())
	}

	if len(body) == 0 {
		return nil, api.NewErrorFields(api.ErrStatusBadRequest, "no transaction found - empty request body")
	}

	var txHex []byte

	switch contentType {
	case "text/plain":
		txHex, err = hex.DecodeString(string(body))
		if err != nil {
			return nil, api.NewErrorFields(api.ErrStatusBadRequest, err.Error())
		}
	case "application/json":
		var txBody api.POSTTransactionJSONRequestBody
		if err = json.Unmarshal(body, &txBody); err != nil {
			return nil, api.NewErrorFields(api.ErrStatusBadRequest, err.Error())
		}

		txHex, err = hex.DecodeString(txBody.RawTx)
		if err != nil {
			return nil, api.NewErrorFields(api.ErrStatusBadRequest, err.Error())
		}
	case "application/octet-stream":
		txHex = body
	default:
		return nil, api.NewErrorFields(api.ErrStatusBadRequest, fmt.Sprintf("given content-type %s does not match any of the allowed content-types", contentType))
	}

	return txHex, nil
}

func parseTransactionsFromRequest(request *http.Request) ([]byte, *api.ErrorFields) {
	requestBody := request.Body
	contentType := request.Header.Get("Content-Type")

	body, err := io.ReadAll(requestBody)
	if err != nil {
		return nil, api.NewErrorFields(api.ErrStatusBadRequest, err.Error())
	}

	if len(body) == 0 {
		return nil, api.NewErrorFields(api.ErrStatusBadRequest, "no transaction found - empty request body")
	}

	var txHex []byte

	switch contentType {
	case "text/plain":
		strBody := strings.ReplaceAll(string(body), "\n", "")
		txHex, err = hex.DecodeString(strBody)
		if err != nil {
			return nil, api.NewErrorFields(api.ErrStatusBadRequest, err.Error())
		}
	case "application/json":
		var txBody api.POSTTransactionsJSONRequestBody
		if err = json.Unmarshal(body, &txBody); err != nil {
			return nil, api.NewErrorFields(api.ErrStatusBadRequest, err.Error())
		}

		for _, tx := range txBody {
			partialHex, err := hex.DecodeString(tx.RawTx)
			if err != nil {
				return nil, api.NewErrorFields(api.ErrStatusBadRequest, err.Error())
			}
			txHex = append(txHex, partialHex...)
		}
	case "application/octet-stream":
		txHex = body
	default:
		return nil, api.NewErrorFields(api.ErrStatusBadRequest, fmt.Sprintf("given content-type %s does not match any of the allowed content-types", contentType))
	}

	return txHex, nil
}
