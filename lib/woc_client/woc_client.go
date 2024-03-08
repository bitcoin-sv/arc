package woc_client

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/bscript"
)

type WocClient struct {
	client http.Client
}

func New() WocClient {
	return WocClient{
		client: http.Client{Timeout: 5 * time.Second},
	}
}

type wocUtxo struct {
	Txid     string `json:"tx_hash"`
	Vout     uint32 `json:"tx_pos"`
	Height   uint32 `json:"height"`
	Satoshis uint64 `json:"value"`
}

type wocBalance struct {
	Confirmed   int64 `json:"confirmed"`
	Unconfirmed int64 `json:"unconfirmed"`
}

func (w *WocClient) GetUTXOs(mainnet bool, lockingScript *bscript.Script, address string) ([]*bt.UTXO, error) {
	// Get UTXOs from WhatsOnChain
	net := "test"
	if mainnet {
		net = "main"
	}
	resp, err := w.client.Get(fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/address/%s/unspent", net, address))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, errors.New("failed to get utxos")
	}

	var wocUnspent []*wocUtxo
	err = json.NewDecoder(resp.Body).Decode(&wocUnspent)
	if err != nil {
		return nil, err
	}

	unspent := make([]*bt.UTXO, len(wocUnspent))
	for i, utxo := range wocUnspent {
		txIDBytes, err := hex.DecodeString(utxo.Txid)
		if err != nil {
			return nil, err
		}

		unspent[i] = &bt.UTXO{
			TxID:          txIDBytes,
			Vout:          utxo.Vout,
			LockingScript: lockingScript,
			Satoshis:      utxo.Satoshis,
		}
	}

	return unspent, nil
}

func (w *WocClient) GetBalance(mainnet bool, address string) (int64, error) {
	net := "test"
	if mainnet {
		net = "main"
	}
	resp, err := http.Get(fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/address/%s/balance", net, address))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, errors.New("failed to get balance")
	}

	var balance wocBalance
	err = json.NewDecoder(resp.Body).Decode(&balance)
	if err != nil {
		return 0, err
	}

	return balance.Confirmed + balance.Unconfirmed, nil
}
