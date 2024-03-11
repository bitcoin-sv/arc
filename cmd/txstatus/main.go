package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"

	"github.com/bitcoin-sv/arc/internal/metamorph"
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

	viper.SetConfigName("config/config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("../../")
	err := viper.ReadInConfig()
	if err != nil {
		fmt.Printf("failed to read config file config.yaml: %v \n", err)
		return
	}

	metamorphAddress := viper.GetString("metamorph.dialAddr")
	if metamorphAddress == "" {
		panic("Missing metamorph.dialAddr")
	}

	grpcMessageSize := viper.GetInt("grpcMessageSize")
	if grpcMessageSize == 0 {
		panic("Missing grpcMessageSize")
	}

	mtmConn, err := metamorph.DialGRPC(metamorphAddress, grpcMessageSize)
	if err != nil {
		panic(err)
	}

	metamorphClient := metamorph.NewClient(metamorph_api.NewMetaMorphAPIClient(mtmConn))

	var res *metamorph.TransactionStatus
	res, err = metamorphClient.GetTransactionStatus(ctx, txid)
	if err != nil {
		panic(err)
	}

	type response struct {
		*metamorph.TransactionStatus
		TransactionBytes string    `json:"transactionBytes"`
		Timestamp        time.Time `json:"timestamp"`
	}

	transactionBytes, err := metamorphClient.GetTransaction(ctx, txid)
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
