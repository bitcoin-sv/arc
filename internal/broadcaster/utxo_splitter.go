package broadcaster

import (
	"errors"
	"log/slog"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"

	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/pkg/keyset"
)

type UTXOSplitter struct {
	Broadcaster
	fromKeySet *keyset.KeySet
	toKeySets  []*keyset.KeySet
}

func NewUTXOSplitter(logger *slog.Logger, client ArcClient, fromKeySet *keyset.KeySet, toKeySets []*keyset.KeySet, isTestnet bool, opts ...func(p *Broadcaster)) (*UTXOSplitter, error) {
	b, err := NewBroadcaster(logger, client, nil, isTestnet, opts...)
	if err != nil {
		return nil, err
	}

	creator := &UTXOSplitter{
		Broadcaster: b,
		toKeySets:   toKeySets,
		fromKeySet:  fromKeySet,
	}

	return creator, nil
}

func (b *UTXOSplitter) SplitUtxo(txid string, satoshis uint64, vout uint32, dryrun bool) error {
	toAddresses := make([]string, len(b.toKeySets))
	for i, key := range b.toKeySets {
		toAddresses[i] = key.Address(!b.isTestnet)
	}

	b.logger.Info("Splitting utxo", slog.String("txid", txid), slog.String("from", b.fromKeySet.Address(!b.isTestnet)), "to", toAddresses)

	var err error
	hash, err := chainhash.NewHashFromHex(txid)
	if err != nil {
		return err
	}
	utxo := &sdkTx.UTXO{
		TxID:          hash,
		Vout:          vout,
		LockingScript: b.fromKeySet.Script,
		Satoshis:      satoshis,
	}

	tx := sdkTx.NewTransaction()
	err = tx.AddInputsFromUTXOs(utxo)
	if err != nil {
		return err
	}

	totalLength := len(b.toKeySets)
	payPerKeyset := satoshis / uint64(totalLength)

	var fee uint64
	for i, toKs := range b.toKeySets {
		if i == len(b.toKeySets)-1 {
			fee, err = ComputeFee(tx, b.feeModel)
			if err != nil {
				return err
			}

			err = PayTo(tx, toKs.Script, payPerKeyset-fee)
			if err != nil {
				return err
			}
			continue
		}

		err = PayTo(tx, toKs.Script, payPerKeyset)
		if err != nil {
			return errors.Join(ErrFailedToAddOutput, err)
		}
	}

	err = SignAllInputs(tx, b.fromKeySet.PrivateKey)
	if err != nil {
		return err
	}

	b.logger.Info("Splitting tx", slog.String("txid", tx.TxID().String()), slog.String("rawTx", tx.String()))
	if dryrun {
		return nil
	}

	b.logger.Info("Submit splitting tx", slog.String("txid", tx.TxID().String()))

	resp, err := b.client.BroadcastTransaction(b.ctx, tx, metamorph_api.Status_SEEN_ON_NETWORK, "")
	if err != nil {
		return errors.Join(ErrFailedToBroadcastTx, err)
	}

	b.logger.Info("Splitting tx submitted", slog.String("txid", tx.TxID().String()), slog.String("status", resp.Status.String()))

	return nil
}
