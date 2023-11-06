package testdata

import (
	"encoding/hex"
	"time"

	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

var (
	Block1        = "0000000000000000072be13e375ffd673b1f37b0ec5ecde7b7e15b01f5685d07"
	Block1Hash, _ = chainhash.NewHashFromStr(Block1)
	Block2        = "000000000000020441ac25b0a9a1339ed75ff183a2500508eb8a5e035aeaca39"
	Block2Hash, _ = chainhash.NewHashFromStr(Block2)

	TX1            = "b042f298deabcebbf15355aa3a13c7d7cfe96c44ac4f492735f936f8e50d06f6"
	TX1Hash, _     = chainhash.NewHashFromStr(TX1)
	TX1Raw         = "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff1a0386c40b2f7461616c2e636f6d2f00cf47ad9c7af83836000000ffffffff0117564425000000001976a914522cf9e7626d9bd8729e5a1398ece40dad1b6a2f88ac00000000"
	TX1RawBytes, _ = hex.DecodeString(TX1Raw)

	TX2        = "1a8fda8c35b8fc30885e88d6eb0214e2b3a74c96c82c386cb463905446011fdf"
	TX2Hash, _ = chainhash.NewHashFromStr(TX2)

	TX3        = "3f63399b3d9d94ba9c5b7398b9328dcccfcfd50f07ad8b214e766168c391642b"
	TX3Hash, _ = chainhash.NewHashFromStr(TX3)

	TX4        = "88eab41a8d0b7b4bc395f8f988ea3d6e63c8bc339526fd2f00cb7ce6fd7df0f7"
	TX4Hash, _ = chainhash.NewHashFromStr(TX4)

	Time          = time.Date(2009, 1, 03, 18, 15, 05, 0, time.UTC)
	DefaultPolicy = `{"excessiveblocksize":2000000000,"blockmaxsize":512000000,"maxtxsizepolicy":10000000,"maxorphantxsize":1000000000,"datacarriersize":4294967295,"maxscriptsizepolicy":500000,"maxopsperscriptpolicy":4294967295,"maxscriptnumlengthpolicy":10000,"maxpubkeyspermultisigpolicy":4294967295,"maxtxsigopscountspolicy":4294967295,"maxstackmemoryusagepolicy":100000000,"maxstackmemoryusageconsensus":200000000,"limitancestorcount":10000,"limitcpfpgroupmemberscount":25,"maxmempool":2000000000,"maxmempoolsizedisk":0,"mempoolmaxpercentcpfp":10,"acceptnonstdoutputs":true,"datacarrier":true,"minminingtxfee":5e-7,"maxstdtxvalidationduration":3,"maxnonstdtxvalidationduration":1000,"maxtxchainvalidationbudget":50,"validationclockcpu":true,"minconsolidationfactor":20,"maxconsolidationinputscriptsize":150,"minconfconsolidationinput":6,"minconsolidationinputmaturity":6,"acceptnonstdconsolidationinput":false}`
)
