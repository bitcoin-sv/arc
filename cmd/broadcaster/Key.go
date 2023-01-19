package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/libsv/go-bk/bec"
	"github.com/libsv/go-bk/bip32"
	"github.com/libsv/go-bk/chaincfg"
	"github.com/libsv/go-bt/v2/bscript"
)

type Key struct {
	extendedKey   *bip32.ExtendedKey
	PrivateKey    *bec.PrivateKey
	PublicKey     *bec.PublicKey
	PublicKeyHash []byte
	ScriptPubKey  string
}

type UTXO struct {
	Txid         string
	Vout         uint32
	ScriptPubKey string
	Satoshis     uint64
}

func (u *UTXO) String() string {
	return fmt.Sprintf("%s:%d (%d sats)", u.Txid, u.Vout, u.Satoshis)
}

type wocUtxo struct {
	Txid     string `json:"tx_hash"`
	Vout     uint32 `json:"tx_pos"`
	Height   uint32 `json:"height"`
	Satoshis uint64 `json:"value"`
}

func (k *Key) Address(mainnet bool) string {
	chain := &chaincfg.MainNet
	if !mainnet {
		chain = &chaincfg.TestNet
	}

	return k.extendedKey.Address(chain)
}

func (k *Key) GetUTXOs(mainnet bool) ([]*UTXO, error) {
	// Get UTXOs from WhatsOnChain
	resp, err := http.Get("https://api.whatsonchain.com/v1/bsv/main/address/" + k.Address(mainnet) + "/unspent")
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, errors.New("failed to get utxos")
	}

	var wocUnspent []*wocUtxo
	err = json.NewDecoder(resp.Body).Decode(&wocUnspent)
	if err != nil {
		return nil, err
	}

	unspent := make([]*UTXO, len(wocUnspent))
	for i, wocUtxo := range wocUnspent {
		unspent[i] = &UTXO{
			Txid:         wocUtxo.Txid,
			Vout:         wocUtxo.Vout,
			ScriptPubKey: k.ScriptPubKey,
			Satoshis:     wocUtxo.Satoshis,
		}
	}

	return unspent, nil
}

func GetPrivateKey(derivationPath string) (*Key, error) {
	extendedBytes, err := os.ReadFile("arc.key")
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("arc.key not found. Please create this file with the xpriv you want to use")
		}
		return nil, err
	}

	extendedKey, err := bip32.NewKeyFromString(string(extendedBytes))
	if err != nil {
		return nil, err
	}

	derivedExtendedKey, err := extendedKey.DeriveChildFromPath(derivationPath)
	if err != nil {
		return nil, err
	}

	privateKey, err := derivedExtendedKey.ECPrivKey()
	if err != nil {
		return nil, err
	}

	publicKey := privateKey.PubKey()

	script, err := bscript.NewP2PKHFromPubKeyEC(publicKey)
	if err != nil {
		return nil, err
	}

	return &Key{
		extendedKey:   derivedExtendedKey,
		PrivateKey:    privateKey,
		PublicKey:     publicKey,
		PublicKeyHash: publicKey.SerialiseCompressed(),
		ScriptPubKey:  script.String(),
	}, nil

}
