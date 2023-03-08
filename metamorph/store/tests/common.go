package tests

import (
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

var (
	Tx1           = "2222222222222222222222222222222233333333333333333333333333333333"
	Tx1Hash, _    = chainhash.NewHashFromStr(Tx1)
	Block1        = "0000000000000000000000000000000011111111111111111111111111111111"
	Block1Hash, _ = chainhash.NewHashFromStr(Block1)
	Block2        = "3333333333333333333333333333333344444444444444444444444444444444"
	Block2Hash, _ = chainhash.NewHashFromStr(Block2)
)
