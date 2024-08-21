package keyset

import (
	"context"
	"crypto/rand"
	primitives "github.com/bitcoin-sv/go-sdk/primitives/ec"
	"github.com/bitcoin-sv/go-sdk/script"
	"github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/bitcoin-sv/go-sdk/transaction/template/p2pkh"

	"github.com/bitcoin-sv/arc/internal/woc_client"
	bip32 "github.com/bitcoin-sv/go-sdk/compat/bip32"
	chaincfg "github.com/bitcoin-sv/go-sdk/transaction/chaincfg"
)

type KeySet struct {
	master        *bip32.ExtendedKey
	Path          string
	PrivateKey    *primitives.PrivateKey
	PublicKey     *primitives.PublicKey
	PublicKeyHash []byte
	Script        *script.Script
}

func (k *KeySet) Address(mainnet bool) string {
	addr, err := script.NewAddressFromPublicKey(k.PrivateKey.PubKey(), mainnet)
	if err != nil {
		panic(err)
	}

	return addr.AddressString
}

func New() (*KeySet, error) {
	var seed [64]byte
	_, err := rand.Read(seed[:])
	if err != nil {
		return nil, err
	}

	master, err := bip32.NewMaster(seed[:], &chaincfg.MainNet)
	if err != nil {
		return nil, err
	}

	return NewFromExtendedKey(master, "")
}

func NewFromExtendedKeyStr(extendedKeyStr string, derivationPath string) (*KeySet, error) {
	extendedKey, err := bip32.NewKeyFromString(extendedKeyStr)
	if err != nil {
		return nil, err
	}

	return NewFromExtendedKey(extendedKey, derivationPath)
}

func NewFromExtendedKey(extendedKey *bip32.ExtendedKey, derivationPath string) (*KeySet, error) {
	var err error

	master := extendedKey

	if derivationPath != "" {
		extendedKey, err = extendedKey.DeriveChildFromPath(derivationPath)
		if err != nil {
			return nil, err
		}
	}

	privateKey, err := extendedKey.ECPrivKey()
	if err != nil {
		return nil, err
	}

	publicKey := privateKey.PubKey()

	address, err := script.NewAddressFromPublicKey(publicKey, true)
	if err != nil {
		return nil, err
	}
	p2pkhScript, err := p2pkh.Lock(address)
	if err != nil {
		return nil, err
	}

	return &KeySet{
		master:        master,
		Path:          derivationPath,
		PrivateKey:    privateKey,
		PublicKey:     publicKey,
		PublicKeyHash: publicKey.SerializeCompressed(),
		Script:        p2pkhScript,
	}, nil
}

func (k *KeySet) DeriveChildFromPath(derivationPath string) (*KeySet, error) {
	return NewFromExtendedKey(k.master, derivationPath)
}

func (k *KeySet) GetUTXOs(mainnet bool) ([]*transaction.UTXO, error) {
	// Get UTXOs from WhatsOnChain
	woc := woc_client.New(mainnet)

	return woc.GetUTXOs(context.Background(), k.Script, k.Address(mainnet))
}

type WocBalance struct {
	Confirmed   uint64 `json:"confirmed"`
	Unconfirmed uint64 `json:"unconfirmed"`
}

func (k *KeySet) GetMaster() *bip32.ExtendedKey {
	return k.master
}

func (k *KeySet) GetBalance(mainnet bool) (WocBalance, error) {
	// Get UTXOs from WhatsOnChain
	woc := woc_client.New(mainnet)
	confirmed, unconfirmed, err := woc.GetBalance(context.Background(), k.Address(mainnet))

	return WocBalance{Confirmed: uint64(confirmed), Unconfirmed: uint64(unconfirmed)}, err
}
