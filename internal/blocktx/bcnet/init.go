package bcnet

import (
	"encoding/binary"
	"io"

	"github.com/bitcoin-sv/go-sdk/script"
	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/bitcoin-sv/go-sdk/util"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
)

func init() {
	// override the default wire block handler with our own that streams and stores only the transaction ids
	wire.SetExternalHandler(wire.CmdBlock, func(reader io.Reader, _ uint64, bytesRead int) (int, wire.Message, []byte, error) {
		blockMessage := &BlockMessage{
			Header: &wire.BlockHeader{},
		}

		err := blockMessage.Header.Deserialize(reader)
		if err != nil {
			return bytesRead, nil, nil, err
		}
		bytesRead += 80 // the bitcoin header is always 80 bytes

		var read int64
		var txCount sdkTx.VarInt
		read, err = txCount.ReadFrom(reader)
		if err != nil {
			return bytesRead, nil, nil, err
		}
		bytesRead += int(read)

		blockMessage.TransactionHashes = make([]*chainhash.Hash, txCount)

		var tx *sdkTx.Transaction
		var hash *chainhash.Hash
		var txBytes []byte
		for i := uint64(0); i < uint64(txCount); i++ {
			tx = sdkTx.NewTransaction()
			read, err = tx.ReadFrom(reader)
			if err != nil {
				return bytesRead, nil, nil, err
			}
			bytesRead += int(read)
			txBytes = tx.TxIDBytes() // this returns the bytes in BigEndian
			hash, err = chainhash.NewHash(util.ReverseBytes(txBytes))
			if err != nil {
				return 0, nil, nil, err
			}

			blockMessage.TransactionHashes[i] = hash

			if i == 0 {
				blockMessage.Height = extractHeightFromCoinbaseTx(tx)
			}
		}

		blockMessage.Size = uint64(bytesRead) // #nosec G115
		blockHash := blockMessage.Header.BlockHash()
		blockMessage.Hash = &blockHash

		return bytesRead, blockMessage, nil, nil
	})
}

func extractHeightFromCoinbaseTx(tx *sdkTx.Transaction) uint64 {
	// Coinbase tx has a special format, the height is encoded in the first 4 bytes of the scriptSig (BIP-34)

	cscript := *(tx.Inputs[0].UnlockingScript)

	if cscript[0] >= script.Op1 && cscript[0] <= script.Op16 { // first 16 blocks are treated differently (we have to handle it due to our tests)
		return uint64(cscript[0] - 0x50)
	}

	hLenght := int(cscript[0])
	if len(cscript) < hLenght+1 {
		return 0
	}

	b := make([]byte, 8)
	for i := 0; i < hLenght; i++ {
		b[i] = cscript[i+1]
	}

	return binary.LittleEndian.Uint64(b)
}
