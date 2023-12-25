package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/bitcoin-sv/arc/api/transactionHandler"
	"github.com/bitcoin-sv/arc/blocktx"
	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/spf13/viper"
)

func main() {
	argsWithoutProg := os.Args[1:]
	var txid string
	if len(argsWithoutProg) == 0 {
		panic("Missing txid")
	}
	txid = argsWithoutProg[0]

	ctx := context.Background()

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("../../")
	err := viper.ReadInConfig()
	if err != nil {
		fmt.Printf("failed to read config file config.yaml: %v \n", err)
		return
	}

	addresses := viper.GetString("metamorph.dialAddr")
	if addresses == "" {
		panic("Missing metamorph.dialAddr")
	}

	btxAddress := viper.GetString("blocktx.dialAddr")
	if btxAddress == "" {
		panic("Missing blocktx.dialAddr")
	}

	conn, err := blocktx.DialGRPC(btxAddress)
	if err != nil {
		panic("failed to connect to block-tx server")
	}

	bTx := blocktx.NewClient(blocktx_api.NewBlockTxAPIClient(conn))
	grpcMessageSize := viper.GetInt("grpcMessageSize")
	if grpcMessageSize == 0 {
		panic("Missing grpcMessageSize")
	}

	txHandler, err := transactionHandler.NewMetamorph(addresses, bTx, grpcMessageSize, false)
	if err != nil {
		panic(err)
	}

	var res *transactionHandler.TransactionStatus
	res, err = txHandler.GetTransactionStatus(ctx, txid)
	if err != nil {
		panic(err)
	}

	type response struct {
		*transactionHandler.TransactionStatus
		TransactionBytes string    `json:"transactionBytes"`
		Timestamp        time.Time `json:"timestamp"`
	}

	transactionBytes, err := txHandler.GetTransaction(ctx, txid)
	if err != nil {
		panic(err)
	}

	r := &response{
		TransactionStatus: res,
		TransactionBytes:  hex.EncodeToString(transactionBytes),
		Timestamp:         time.Unix(res.Timestamp, 0).UTC(),
	}

	var b []byte
	b, err = json.MarshalIndent(r, "", "  ")
	if err != nil {
		panic(err)
	}

	fmt.Printf("%s\n", b)
}
