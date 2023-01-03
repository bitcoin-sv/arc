package blocktx

import (
	"testing"

	"github.com/TAAL-GmbH/arc/p2p"
	"github.com/TAAL-GmbH/arc/p2p/chaincfg/chainhash"
	"github.com/TAAL-GmbH/arc/p2p/wire"
)

func TestGetBlocks(t *testing.T) {

	store := NewBlockTxPeerStore()

	var zeroHash chainhash.Hash

	msg := wire.NewMsgGetBlocks(&zeroHash)

	hash, err := chainhash.NewHashFromStr("2f2c1a204f8406b47e64f750d8b1b098de841ab7b403b0bd685bca5b1ec8cd6c")
	if err != nil {
		t.Fatal(err)
	}

	if err := msg.AddBlockLocatorHash(hash); err != nil {
		t.Fatal(err)
	}

	pm := p2p.NewPeerManager(nil)
	pm.AddPeer("localhost:18333", store)

	pm.GetPeers()[0].WriteChan() <- msg

	ch := make(chan bool)
	<-ch
}
