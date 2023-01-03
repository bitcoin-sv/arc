package blocktx

import (
	"testing"

	"github.com/TAAL-GmbH/arc/p2p"
	"github.com/TAAL-GmbH/arc/p2p/chaincfg/chainhash"
	"github.com/TAAL-GmbH/arc/p2p/wire"
)

func TestGetBlocks(t *testing.T) {

	store := NewBlockTxPeerStore()

	//var zeroHash chainhash.Hash
	hash, err := chainhash.NewHashFromStr("232b2b7ea63ff7a2f070a9a6800d3b1f1af8b8226e2fc76c27593c4e54d4ba23")
	if err != nil {
		t.Fatal(err)
	}

	// this should be the earliest block we want to get
	msg := wire.NewMsgGetBlocks(hash)

	// add block hashes we know about
	if err = msg.AddBlockLocatorHash(hash); err != nil {
		t.Fatal(err)
	}

	pm := p2p.NewPeerManager(nil)
	_ = pm.AddPeer("localhost:18333", store)

	pm.GetPeers()[0].WriteMsg(msg)

	ch := make(chan bool)
	<-ch
}
