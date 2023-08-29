package handler

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"

	"github.com/bitcoin-sv/arc/api/transactionHandler"
	"github.com/opentracing/opentracing-go"
	"github.com/ordishs/go-bitcoin"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

func getTransactionFromNode(ctx context.Context, inputTxID string) ([]byte, error) {
	span, _ := opentracing.StartSpanFromContext(ctx, "getTransactionFromNode")
	defer span.Finish()

	peerRpcPassword := viper.GetString("peerRpcPassword")
	if peerRpcPassword == "" {
		return nil, errors.Errorf("setting peerRpcPassword not found")
	}

	peerRpcUser := viper.GetString("peerRpcUser")
	if peerRpcUser == "" {
		return nil, errors.Errorf("setting peerRpcUser not found")
	}

	peerRpcHost := viper.GetString("peerRpcHost")
	if peerRpcHost == "" {
		return nil, errors.Errorf("setting peerRpcHost not found")
	}

	peerRpcPort := viper.GetInt("peerRpcPort")
	if peerRpcPort == 0 {
		return nil, errors.Errorf("setting peerRpcPort not found")
	}

	// get the transaction from the bitcoin node rpc
	node, err := bitcoin.New(peerRpcHost, peerRpcPort, peerRpcUser, peerRpcPassword, false)
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

	return nil, transactionHandler.ErrParentTransactionNotFound
}

func getTransactionFromWhatsOnChain(ctx context.Context, inputTxID string) ([]byte, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "getTransactionFromWhatsOnChain")
	defer span.Finish()

	wocApiKey := viper.GetString("wocApiKey")

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
		return nil, transactionHandler.ErrParentTransactionNotFound
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

	return nil, transactionHandler.ErrParentTransactionNotFound
}
