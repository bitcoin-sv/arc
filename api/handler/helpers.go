package handler

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/bitcoin-sv/arc/api"
	"github.com/bitcoin-sv/arc/api/dictionary"
	"github.com/bitcoin-sv/arc/api/transaction_handler"
	"github.com/deepmap/oapi-codegen/pkg/middleware"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/labstack/echo/v4"
	"github.com/opentracing/opentracing-go"
	"github.com/ordishs/go-bitcoin"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

func getTransactionFromNode(ctx context.Context, inputTxID string) ([]byte, error) {
	span, _ := opentracing.StartSpanFromContext(ctx, "getTransactionFromNode")
	defer span.Finish()

	peerRpcPassword := viper.GetString("peerRpc.password")
	if peerRpcPassword == "" {
		return nil, errors.Errorf("setting peerRpc.password not found")
	}

	peerRpcUser := viper.GetString("peerRpc.user")
	if peerRpcUser == "" {
		return nil, errors.Errorf("setting peerRpc.user not found")
	}

	peerRpcHost := viper.GetString("peerRpc.host")
	if peerRpcHost == "" {
		return nil, errors.Errorf("setting peerRpc.host not found")
	}

	peerRpcPort := viper.GetInt("peerRpc.port")
	if peerRpcPort == 0 {
		return nil, errors.Errorf("setting peerRpc.port not found")
	}

	rpcURL, err := url.Parse(fmt.Sprintf("rpc://%s:%s@%s:%d", peerRpcUser, peerRpcPassword, peerRpcHost, peerRpcPort))
	if err != nil {
		return nil, errors.Errorf("failed to parse rpc URL: %v", err)
	}
	// get the transaction from the bitcoin node rpc
	node, err := bitcoin.NewFromURL(rpcURL, false)
	if err != nil {
		return nil, err
	}

	var tx *bitcoin.RawTransaction
	tx, err = node.GetRawTransaction(inputTxID)
	if err != nil {
		return nil, err
	}

	var txBytes []byte
	txBytes, err = hex.DecodeString(tx.Hex)
	if err != nil {
		return nil, err
	}

	if txBytes != nil {
		return txBytes, nil
	}

	return nil, transaction_handler.ErrParentTransactionNotFound
}

func getTransactionFromWhatsOnChain(ctx context.Context, inputTxID string) ([]byte, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "getTransactionFromWhatsOnChain")
	defer span.Finish()

	wocApiKey := viper.GetString("api.wocApiKey")

	if wocApiKey == "" {
		return nil, errors.Errorf("setting wocApiKey not found")
	}

	wocURL := fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/tx/%s/hex", "main", inputTxID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wocURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", wocApiKey)

	var resp *http.Response
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, transaction_handler.ErrParentTransactionNotFound
	}

	var txHexBytes []byte
	txHexBytes, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	txHex := string(txHexBytes)

	var txBytes []byte
	txBytes, err = hex.DecodeString(txHex)
	if err != nil {
		return nil, err
	}

	if txBytes != nil {
		return txBytes, nil
	}

	return nil, transaction_handler.ErrParentTransactionNotFound
}

// To returns a pointer to the given value.
func PtrTo[T any](v T) *T {
	return &v
}

func GetDefaultPolicy() (*bitcoin.Settings, error) {
	defaultPolicy := &bitcoin.Settings{}

	err := viper.UnmarshalKey("api.defaultPolicy", defaultPolicy)
	if err != nil {
		return nil, err
	}

	return defaultPolicy, nil
}

// CheckSwagger validates the request against the swagger definition.
func CheckSwagger(e *echo.Echo) *openapi3.T {
	swagger, err := api.GetSwagger()
	if err != nil {
		log.Fatalf(dictionary.GetInternalMessage(dictionary.ErrorLoadingSwaggerSpec), err.Error())
	}

	// Clear out the servers array in the swagger spec, that skips validating
	// that server names match. We don't know how this thing will be run.
	swagger.Servers = nil
	// Clear out the security requirements, we check this ourselves
	swagger.Security = nil

	// Use our validation middleware to check all requests against the OpenAPI schema.
	e.Use(middleware.OapiRequestValidator(swagger))

	return swagger
}
