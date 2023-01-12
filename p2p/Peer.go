package p2p

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/TAAL-GmbH/arc/p2p/bsvutil"
	"github.com/TAAL-GmbH/arc/p2p/wire"

	"github.com/ordishs/go-utils"
)

var (
	pingInterval = 2 * time.Minute
)

type Block struct {
	Hash         []byte `json:"hash,omitempty"`          // Little endian
	PreviousHash []byte `json:"previous_hash,omitempty"` // Little endian
	MerkleRoot   []byte `json:"merkle_root,omitempty"`   // Little endian
	Height       uint64 `json:"height,omitempty"`
}

type Peer struct {
	address     string
	network     wire.BitcoinNet
	conn        net.Conn
	peerHandler PeerHandlerI
	writeChan   chan wire.Message
	quit        chan struct{}
	logger      utils.Logger
	initialized sync.WaitGroup
}

// NewPeer returns a new bitcoin peer for the provided address and configuration.
func NewPeer(logger utils.Logger, address string, peerHandler PeerHandlerI, network wire.BitcoinNet) (*Peer, error) {
	writeChan := make(chan wire.Message, 100)

	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("could not dial node [%s]: %v", address, err)
	}

	p := &Peer{
		conn:        conn,
		network:     network,
		address:     address,
		writeChan:   writeChan,
		peerHandler: peerHandler,
		logger:      logger,
		initialized: sync.WaitGroup{},
	}

	// create the init lock for Version and VerAck
	p.initialized.Add(2)

	go p.readHandler()
	go p.pingHandler()
	go p.writeChannelHandler()

	// write version message to our peer directly and not through the write channel,
	// write channel is not ready to send message until the VERACK handshake is done
	msg := p.versionMessage(address)

	if err = wire.WriteMessage(p.conn, msg, wire.ProtocolVersion, network); err != nil {
		return nil, fmt.Errorf("failed to write message: %v", err)
	}
	p.LogInfo("Sent %s", strings.ToUpper(msg.Command()))

	return p, nil
}

func (p *Peer) WriteMsg(msg wire.Message) {
	p.writeChan <- msg
}

func (p *Peer) String() string {
	return p.address
}

func (p *Peer) LogError(format string, args ...interface{}) {
	p.logger.Errorf("[%s] "+format, append([]interface{}{p.address}, args...)...)
}

func (p *Peer) LogInfo(format string, args ...interface{}) {
	p.logger.Infof("[%s] "+format, append([]interface{}{p.address}, args...)...)
}

func (p *Peer) readHandler() {
	for {
		msg, b, err := wire.ReadMessage(p.conn, wire.ProtocolVersion, p.network)
		if err != nil {
			if errors.Is(err, io.EOF) {
				p.LogError(fmt.Sprintf("READ EOF whilst reading from %s [%d bytes]\n%s", p.address, len(b), string(b)))
				break
			}
			p.LogError("Failed to read message: %v", err)
			continue
		}

		switch msg.Command() {
		case wire.CmdVersion:
			p.LogInfo("Recv %s", strings.ToUpper(msg.Command()))
			verackMsg := wire.NewMsgVerAck()
			if err = wire.WriteMessage(p.conn, verackMsg, wire.ProtocolVersion, p.network); err != nil {
				p.LogError("failed to write message: %v", err)
			}
			p.LogInfo("Sent %s", strings.ToUpper(verackMsg.Command()))
			p.initialized.Done()

		case wire.CmdPing:
			pingMsg := msg.(*wire.MsgPing)
			p.writeChan <- wire.NewMsgPong(pingMsg.Nonce)

		case wire.CmdInv:
			invMsg := msg.(*wire.MsgInv)
			p.LogInfo("Recv INV (%d items)", len(invMsg.InvList))
			for _, inv := range invMsg.InvList {
				p.LogInfo("        %s", inv.Hash.String())
			}

			go func(invList []*wire.InvVect) {
				for _, invVect := range invList {
					switch invVect.Type {
					case wire.InvTypeTx:
						if err = p.peerHandler.HandleTransactionAnnouncement(invVect, p); err != nil {
							p.LogError("Unable to process tx %s: %v", invVect.Hash.String(), err)
						}
					case wire.InvTypeBlock:
						if err = p.peerHandler.HandleBlockAnnouncement(invVect, p); err != nil {
							p.LogError("Unable to process block %s: %v", invVect.Hash.String(), err)
						}
					}
				}
			}(invMsg.InvList)

		case wire.CmdGetData:
			dataMsg := msg.(*wire.MsgGetData)
			p.LogInfo("Recv GETDATA (%d items)", len(dataMsg.InvList))
			for _, inv := range dataMsg.InvList {
				p.LogInfo("        %s", inv.Hash.String())
			}
			p.handleGetDataMsg(dataMsg)

		case wire.CmdBlock:
			blockMsg := msg.(*wire.MsgBlock)
			p.LogInfo("Recv %s: %s", strings.ToUpper(msg.Command()), blockMsg.BlockHash().String())

			err = p.peerHandler.HandleBlock(blockMsg, p)
			if err != nil {
				p.LogError("Unable to process block %s: %v", blockMsg.BlockHash().String(), err)
			}

			// read the remainder of the block, if not consumed by the handler
			// TODO is this necessary or can we just ignore whether the reader has been consumed?
			_, _ = io.ReadAll(blockMsg.TransactionReader)

		case wire.CmdReject:
			rejMsg := msg.(*wire.MsgReject)
			if err = p.peerHandler.HandleTransactionRejection(rejMsg, p); err != nil {
				p.LogError("Unable to process block %s: %v", rejMsg.Hash.String(), err)
			}

		case wire.CmdVerAck:
			p.LogInfo("Recv %s", strings.ToUpper(msg.Command()))
			p.initialized.Done()

		default:
			p.logger.Warnf("Ignored %s", strings.ToUpper(msg.Command()))
		}
	}
}

func (p *Peer) handleGetDataMsg(dataMsg *wire.MsgGetData) {
	for _, invVect := range dataMsg.InvList {
		switch invVect.Type {
		case wire.InvTypeTx:
			p.LogInfo("Request for TX: %s\n", invVect.Hash.String())

			txBytes, err := p.peerHandler.GetTransactionBytes(invVect)
			if err != nil {
				p.LogError("Unable to fetch tx %s from store: %v", invVect.Hash.String(), err)
				continue
			}

			if txBytes == nil {
				p.logger.Warnf("Unable to fetch tx %s from store: %v", invVect.Hash.String(), err)
				continue
			}

			tx, err := bsvutil.NewTxFromBytes(txBytes)
			if err != nil {
				log.Print(err) // Log and handle the error
				continue
			}

			p.writeChan <- tx.MsgTx()

		case wire.InvTypeBlock:
			p.LogInfo("Request for Block: %s\n", invVect.Hash.String())

		default:
			p.logger.Warnf("Unknown type: %d\n", invVect.Type)
		}
	}
}

func (p *Peer) writeChannelHandler() {
	p.initialized.Wait() // wait to send new messages until we are initialized

	for msg := range p.writeChan {
		if err := wire.WriteMessage(p.conn, msg, wire.ProtocolVersion, p.network); err != nil {
			if errors.Is(err, io.EOF) {
				panic("WRITE EOF")
			}
			p.LogError("Failed to write message: %v", err)
		}

		if msg.Command() == wire.CmdTx {
			hash := msg.(*wire.MsgTx).TxHash()
			if err := p.peerHandler.HandleTransactionSent(msg.(*wire.MsgTx), p); err != nil {
				p.LogError("Unable to process tx %s: %v", hash.String(), err)
			}
		}

		switch m := msg.(type) {
		case *wire.MsgTx:
			p.LogInfo("Sent %s: %s", strings.ToUpper(msg.Command()), m.TxHash().String())
		case *wire.MsgBlock:
			p.LogInfo("Sent %s: %s", strings.ToUpper(msg.Command()), m.BlockHash().String())
		case *wire.MsgGetData:
			p.LogInfo("Sent %s: %s", strings.ToUpper(msg.Command()), m.InvList[0].Hash.String())
		case *wire.MsgInv:
		default:
			p.LogInfo("Sent %s", strings.ToUpper(msg.Command()))
		}
	}
}

func (p *Peer) versionMessage(address string) *wire.MsgVersion {
	lastBlock := int32(0)

	tcpAddrMe := &net.TCPAddr{IP: nil, Port: 0}
	me := wire.NewNetAddress(tcpAddrMe, wire.SFNodeNetwork)

	parts := strings.Split(address, ":")
	if len(parts) != 2 {
		panic(fmt.Sprintf("Could not parse address %s", address))
	}

	port, err := strconv.Atoi(parts[1])
	if err != nil {
		panic(fmt.Sprintf("Could not parse port %s", parts[1]))
	}

	tcpAddrYou := &net.TCPAddr{IP: net.ParseIP(parts[0]), Port: port}
	you := wire.NewNetAddress(tcpAddrYou, wire.SFNodeNetwork)

	nonce, err := wire.RandomUint64()
	if err != nil {
		p.logger.Errorf("RandomUint64: error generating nonce: %v", err)
	}

	msg := wire.NewMsgVersion(me, you, nonce, lastBlock)

	return msg
}

// pingHandler periodically pings the peer.  It must be run as a goroutine.
func (p *Peer) pingHandler() {
	pingTicker := time.NewTicker(pingInterval)
	defer pingTicker.Stop()

out:
	for {
		select {
		case <-pingTicker.C:
			nonce, err := wire.RandomUint64()
			if err != nil {
				p.LogError("Not sending ping to %s: %v", p, err)
				continue
			}
			p.writeChan <- wire.NewMsgPing(nonce)

		case <-p.quit:
			break out
		}
	}
}
