package p2p

import (
	"io"

	"github.com/TAAL-GmbH/arc/p2p/wire"
	"github.com/libsv/go-bt/v2"
)

func setPeerBlockHandler() {
	wire.SetExternalHandler(wire.CmdBlock, func(reader io.Reader, length uint64, bytesRead int) (int, wire.Message, []byte, error) {
		blockMessage := &BlockMessage{
			Header: &wire.BlockHeader{},
		}

		err := blockMessage.Header.Deserialize(reader)
		if err != nil {
			return bytesRead, nil, nil, err
		}
		bytesRead += 80 // the bitcoin header is always 80 bytes

		var read int64
		var txCount bt.VarInt
		read, err = txCount.ReadFrom(reader)
		if err != nil {
			return bytesRead, nil, nil, err
		}
		bytesRead += int(read)

		for i := 0; i < int(txCount); i++ {
			tx := bt.NewTx()
			read, err = tx.ReadFrom(reader)
			if err != nil {
				return bytesRead, nil, nil, err
			}
			bytesRead += int(read)
			txBytes := tx.TxIDBytes() // this returns the bytes in BigEndian
			blockMessage.TransactionIDs = append(blockMessage.TransactionIDs, bt.ReverseBytes(txBytes))

			if i == 0 {
				blockMessage.Height = extractHeightFromCoinbaseTx(tx)
			}
		}

		blockMessage.Size = uint64(bytesRead)

		return bytesRead, blockMessage, nil, nil
	})
}
