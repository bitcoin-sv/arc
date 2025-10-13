package testdata

import (
	"time"

	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"

	"github.com/bsv-blockchain/go-bt/v2/chainhash"
)

var (
	Block1B        = "0000000000000000072be13e375ffd673b1f37b0ec5ecde7b7e15b01f5685d07"
	Block1HashB, _ = chainhash.NewHashFromStr(Block1)
	Block2B        = "000000000000020441ac25b0a9a1339ed75ff183a2500508eb8a5e035aeaca39"
	Block2HashB, _ = chainhash.NewHashFromStr(Block2)

	TX1RawStringB = "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff1a0386c40b2f7461616c2e636f6d2f00cf47ad9c7af83836000000ffffffff0117564425000000001976a914522cf9e7626d9bd8729e5a1398ece40dad1b6a2f88ac00000000"
	TX1RawB, _    = sdkTx.NewTransactionFromHex(TX1RawString)
	TX1HashB, _   = chainhash.NewHashFromStr(TX1Raw.TxID().String())

	ValidTXRawStringB = "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff1a0386c40b2f7461616c2e636f6d2f00cf47ad9c7af83836000000ffffffff0117564425000000001976a914522cf9e7626d9bd8729e5a1398ece40dad1b6a2f88ac00000000"
	ValidTXRawB, _    = sdkTx.NewTransactionFromHex(ValidTXRawString)
	ValidTXHashB, _   = chainhash.NewHashFromStr(ValidTXRaw.TxID().String())

	TX2B        = "1a8fda8c35b8fc30885e88d6eb0214e2b3a74c96c82c386cb463905446011fdf"
	TX2HashB, _ = chainhash.NewHashFromStr(TX2)

	TX3B        = "3f63399b3d9d94ba9c5b7398b9328dcccfcfd50f07ad8b214e766168c391642b"
	TX3HashB, _ = chainhash.NewHashFromStr(TX3)

	TX4B        = "88eab41a8d0b7b4bc395f8f988ea3d6e63c8bc339526fd2f00cb7ce6fd7df0f7"
	TX4HashB, _ = chainhash.NewHashFromStr(TX4)

	TX5B        = "df931ab7d4ff0bbf96ff186f221c466f09c052c5331733641040defabf9dcd93"
	TX5HashB, _ = chainhash.NewHashFromStr(TX5)

	TX6RawStringB = "010000000000000000ef016f8828b2d3f8085561d0b4ff6f5d17c269206fa3d32bcd3b22e26ce659ed12e7000000006b483045022100d3649d120249a09af44b4673eecec873109a3e120b9610b78858087fb225c9b9022037f16999b7a4fecdd9f47ebdc44abd74567a18940c37e1481ab0fe84d62152e4412102f87ce69f6ba5444aed49c34470041189c1e1060acd99341959c0594002c61bf0ffffffffe7030000000000001976a914c2b6fd4319122b9b5156a2a0060d19864c24f49a88ac01e7030000000000001976a914c2b6fd4319122b9b5156a2a0060d19864c24f49a88ac00000000"
	TX6RawB, _    = sdkTx.NewTransactionFromHex(TX6RawString)
	TX6HashB, _   = chainhash.NewHashFromStr(TX6Raw.TxID().String())

	Time          = time.Date(2009, 1, 0o3, 18, 15, 0o5, 0, time.UTC)
	DefaultPolicy = `{"excessiveblocksize":2000000000,"blockmaxsize":512000000,"maxtxsizepolicy":10000000,"maxorphantxsize":1000000000,"datacarriersize":4294967295,"maxscriptsizepolicy":500000,"maxopsperscriptpolicy":4294967295,"maxscriptnumlengthpolicy":10000,"maxpubkeyspermultisigpolicy":4294967295,"maxtxsigopscountspolicy":4294967295,"maxstackmemoryusagepolicy":100000000,"maxstackmemoryusageconsensus":200000000,"limitancestorcount":10000,"limitcpfpgroupmemberscount":25,"maxmempool":2000000000,"maxmempoolsizedisk":0,"mempoolmaxpercentcpfp":10,"acceptnonstdoutputs":true,"datacarrier":true,"minminingtxfee":5e-7,"maxstdtxvalidationduration":3,"maxnonstdtxvalidationduration":1000,"maxtxchainvalidationbudget":50,"validationclockcpu":true,"minconsolidationfactor":20,"maxconsolidationinputscriptsize":150,"minconfconsolidationinput":6,"minconsolidationinputmaturity":6,"acceptnonstdconsolidationinput":false}`
)
