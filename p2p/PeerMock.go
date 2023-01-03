package p2p

import (
	"sync"

	"github.com/TAAL-GmbH/arc/p2p/wire"
)

type PeerMock struct {
	mu            sync.Mutex
	address       string
	peerStore     PeerStoreI
	parentChannel chan *PMMessage
	writeChan     chan wire.Message
	messages      []wire.Message
}

func NewPeerMock(address string, peerStore PeerStoreI, parentChannel chan *PMMessage) (*PeerMock, error) {
	writeChan := make(chan wire.Message)

	p := &PeerMock{
		peerStore:     peerStore,
		address:       address,
		writeChan:     writeChan,
		parentChannel: parentChannel,
	}

	go func() {
		for msg := range writeChan {
			p.message(msg)
		}
	}()

	return p, nil
}

func (p *PeerMock) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return len(p.messages)
}

func (p *PeerMock) message(msg wire.Message) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.messages = append(p.messages, msg)
}

func (p *PeerMock) getMessages() []wire.Message {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.messages
}

func (p *PeerMock) WriteMsg(msg wire.Message) {
	p.writeChan <- msg
}

func (p *PeerMock) String() string {
	return p.address
}

func (p *PeerMock) ReceiveMessage(message *PMMessage) {
	p.parentChannel <- message
}
