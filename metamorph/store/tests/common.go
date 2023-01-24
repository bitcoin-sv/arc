package tests

import "github.com/ordishs/go-utils"

var (
	Tx1            = "b042f298deabcebbf15355aa3a13c7d7cfe96c44ac4f492735f936f8e50d06f6"
	Tx1Bytes, _    = utils.DecodeAndReverseHexString(Tx1)
	Block1         = "a042f298deabcebbf15355aa3a13c7d7cfe96c44ac4f492735f936f8e50d06f7"
	Block1Bytes, _ = utils.DecodeAndReverseHexString(Block1)
)
