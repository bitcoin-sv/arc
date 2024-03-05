package woc_client

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/bscript"
)

type WocClient struct{}

func New() WocClient {
	return WocClient{}
}

type wocUtxo struct {
	Txid     string `json:"tx_hash"`
	Vout     uint32 `json:"tx_pos"`
	Height   uint32 `json:"height"`
	Satoshis uint64 `json:"value"`
}

func (w *WocClient) GetUTXOs(mainnet bool, lockingScript *bscript.Script, address string) ([]*bt.UTXO, error) {
	// Get UTXOs from WhatsOnChain
	net := "test"
	if mainnet {
		net = "main"
	}
	resp, err := http.Get(fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/address/%s/unspent", net, address))
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
