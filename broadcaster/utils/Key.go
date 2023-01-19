package utils

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/libsv/go-bk/bec"
	"github.com/libsv/go-bk/bip32"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/bscript"
)

type Key struct {
	extendedKey   *bip32.ExtendedKey
	PrivateKey    *bec.PrivateKey
	PublicKey     *bec.PublicKey
	PublicKeyHash []byte
	ScriptPubKey  string
}

type wocUtxo struct {
	Txid     string `json:"tx_hash"`
	Vout     uint32 `json:"tx_pos"`
	Height   uint32 `json:"height"`
	Satoshis uint64 `json:"value"`
}

func (k *Key) Address(mainnet bool) string {
	addr, err := bscript.NewAddressFromPublicKey(k.PrivateKey.PubKey(), mainnet)
	if err != nil {
		panic(err)
	}

	return addr.AddressString
}

func (k *Key) GetUTXOs(mainnet bool) ([]*bt.UTXO, error) {
	// Get UTXOs from WhatsOnChain
	net := "test"
	if mainnet {
		net = "main"
	}
	resp, err := http.Get(fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/address/%s/unspent", net, k.Address(mainnet)))
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
		lockingScript, err := bscript.NewFromHexString(k.ScriptPubKey)
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

func NewPrivateKey(privateKey *bec.PrivateKey) (*Key, error) {
	publicKey := privateKey.PubKey()

	script, err := bscript.NewP2PKHFromPubKeyEC(publicKey)
	if err != nil {
		return nil, err
	}

	return &Key{
		PrivateKey:    privateKey,
		PublicKey:     publicKey,
		PublicKeyHash: publicKey.SerialiseCompressed(),
		ScriptPubKey:  script.String(),
	}, nil
}

func GetPrivateKey(xpriv string, derivationPath string) (*Key, error) {
	extendedKey, err := bip32.NewKeyFromString(xpriv)
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
