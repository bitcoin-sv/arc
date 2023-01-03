package p2p

import (
	"sync"

	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/TAAL-GmbH/arc/p2p/wire"
)

type PeerMock struct {
	mu            sync.Mutex
	address       string
	s             store.Store
	parentChannel chan *PMMessage
	writeChan     chan wire.Message
	Messages      []wire.Message
}

func NewPeerMock(address string, s store.Store, parentChannel chan *PMMessage) (*PeerMock, error) {
	writeChan := make(chan wire.Message)

	p := &PeerMock{
		s:             s,
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

func (p *PeerMock) message(msg wire.Message) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Messages = append(p.Messages, msg)
}

func (p *PeerMock) getMessages() []wire.Message {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.Messages
}

func (p *PeerMock) WriteChan() chan wire.Message {
	return p.writeChan
}

func (p *PeerMock) String() string {
	return p.address
}

func (p *PeerMock) ReceiveMessage(message *PMMessage) {
	p.parentChannel <- message
}
