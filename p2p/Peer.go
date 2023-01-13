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
	"sync/atomic"
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
	address        string
	network        wire.BitcoinNet
	mu             sync.RWMutex
	readConn       net.Conn
	writeConn      net.Conn
	peerHandler    PeerHandlerI
	writeChan      chan wire.Message
	quit           chan struct{}
	logger         utils.Logger
	sentVerAck     atomic.Bool
	receivedVerAck atomic.Bool
}

// NewPeer returns a new bitcoin peer for the provided address and configuration.
func NewPeer(logger utils.Logger, address string, peerHandler PeerHandlerI, network wire.BitcoinNet) (*Peer, error) {
	writeChan := make(chan wire.Message, 100)

	p := &Peer{
		network:     network,
		address:     address,
		writeChan:   writeChan,
		peerHandler: peerHandler,
		logger:      logger,
	}

	go p.pingHandler()
	go p.writeChannelHandler()

	// reconnect if disconnected
	go func() {
		for {
			if !p.Connected() {
				err := p.connect()
				if err != nil {
					logger.Warnf("Failed to connect to peer %s: %v", address, err)
				}
			}
			time.Sleep(10 * time.Second)
		}
	}()

	return p, nil
}

func (p *Peer) disconnect() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.readConn != nil {
		_ = p.readConn.Close()
	}

	p.readConn = nil
	p.writeConn = nil
	p.sentVerAck.Store(false)
	p.receivedVerAck.Store(false)
}

func (p *Peer) connect() error {
	p.mu.Lock()
	p.readConn = nil
	p.sentVerAck.Store(false)
	p.receivedVerAck.Store(false)
	p.mu.Unlock()

	p.LogInfo("Connecting to peer on %s", p.network)
	conn, err := net.Dial("tcp", p.address)
	if err != nil {
		return fmt.Errorf("could not dial node [%s]: %v", p.address, err)
	}

	// open the read connection, so we can receive messages
	p.mu.Lock()
	p.readConn = conn
	p.mu.Unlock()

	go p.readHandler()

	// write version message to our peer directly and not through the write channel,
	// write channel is not ready to send message until the VERACK handshake is done
	msg := p.versionMessage(p.address)

	if err = wire.WriteMessage(conn, msg, wire.ProtocolVersion, p.network); err != nil {
		return fmt.Errorf("failed to write message: %v", err)
	}
	p.LogInfo("Sent %s", strings.ToUpper(msg.Command()))

	for {
		if p.receivedVerAck.Load() && p.sentVerAck.Load() {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// set the connection which allows us to send messages
	p.mu.Lock()
	p.writeConn = conn
	p.mu.Unlock()

	return nil
}

func (p *Peer) Connected() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.readConn != nil && p.writeConn != nil
}

func (p *Peer) WriteMsg(msg wire.Message) error {
	p.mu.RLock()
	writeConn := p.writeConn
	p.mu.RUnlock()

	if writeConn == nil {
		return errors.New("peer is not connected")
	}

	p.writeChan <- msg
	return nil
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

func (p *Peer) LogWarn(format string, args ...interface{}) {
	p.logger.Warnf("[%s] "+format, append([]interface{}{p.address}, args...)...)
}

func (p *Peer) readHandler() {
	p.mu.RLock()
	readConn := p.readConn
	p.mu.RUnlock()

	if readConn != nil {
		for {
			msg, b, err := wire.ReadMessage(readConn, wire.ProtocolVersion, p.network)
			if err != nil {
				if errors.Is(err, io.EOF) {
					p.LogError(fmt.Sprintf("READ EOF whilst reading from %s [%d bytes]\n%s", p.address, len(b), string(b)))
					p.disconnect()
					break
				}
				p.LogError("Failed to read message: %v", err)
				continue
			}

			switch msg.Command() {
			case wire.CmdVersion:
				p.LogInfo("Recv %s", strings.ToUpper(msg.Command()))
				verackMsg := wire.NewMsgVerAck()
				if err = wire.WriteMessage(readConn, verackMsg, wire.ProtocolVersion, p.network); err != nil {
					p.LogError("failed to write message: %v", err)
				}
				p.LogInfo("Sent %s", strings.ToUpper(verackMsg.Command()))
				p.sentVerAck.Store(true)

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
				p.receivedVerAck.Store(true)

			default:
				p.LogWarn("Ignored %s", strings.ToUpper(msg.Command()))
			}
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
				p.LogWarn("Unable to fetch tx %s from store: %v", invVect.Hash.String(), err)
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
			p.LogWarn("Unknown type: %d\n", invVect.Type)
		}
	}
}

func (p *Peer) writeChannelHandler() {
	for msg := range p.writeChan {
		// wait for the write connection to be ready
		for {
			p.mu.RLock()
			writeConn := p.writeConn
			p.mu.RUnlock()

			if writeConn != nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		if err := wire.WriteMessage(p.writeConn, msg, wire.ProtocolVersion, p.network); err != nil {
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
		p.LogError("RandomUint64: error generating nonce: %v", err)
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
