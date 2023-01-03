package p2p

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/p2p/wire"

	"github.com/TAAL-GmbH/arc/p2p/bsvutil"

	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"
)

var (
	logger       = gocore.Log("p2p")
	pingInterval = 2 * time.Minute
	magic        = wire.TestNet
)

func init() {
	if gocore.Config().GetBool("mainnet", false) {
		magic = wire.MainNet
	}
}

type PeerStoreI interface {
	GetTransactionBytes(hash []byte) ([]byte, error)
	ProcessBlock(hash []byte) error
}

type Peer struct {
	address       string
	conn          net.Conn
	peerStore     PeerStoreI
	parentChannel chan *PMMessage
	writeChan     chan wire.Message
	quit          chan struct{}
}

func NewPeer(address string, peerStore PeerStoreI) (*Peer, error) {
	writeChan := make(chan wire.Message, 100)

	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("could not dial node [%s]: %v", address, err)
	}

	p := &Peer{
		conn:      conn,
		address:   address,
		writeChan: writeChan,
		peerStore: peerStore,
	}

	go p.readHandler()
	go p.pingHandler()
	go p.writeChannelHandler()

	writeChan <- versionMessage()

	return p, nil
}

func (p *Peer) AddParentMessageChannel(parentChannel chan *PMMessage) *Peer {
	p.parentChannel = parentChannel

	return p
}

func (p *Peer) WriteChan() chan wire.Message {
	return p.writeChan
}

func (p *Peer) String() string {
	return p.address
}

func (p *Peer) readHandler() {
	for {
		msg, _, err := wire.ReadMessage(p.conn, wire.ProtocolVersion, magic)
		if err != nil {
			if errors.Is(err, io.EOF) {
				panic("READ EOF")
			}
			logger.Errorf("Failed to read message: %v", err)
			continue
		}

		switch msg.Command() {
		case wire.CmdVersion:
			p.writeChan <- wire.NewMsgVerAck()

		case wire.CmdPing:
			pingMsg := msg.(*wire.MsgPing)
			p.writeChan <- wire.NewMsgPong(pingMsg.Nonce)

		case wire.CmdInv:
			invMsg := msg.(*wire.MsgInv)
			logger.Infof("Recv INV (%d items)", len(invMsg.InvList))
			go func(invList []*wire.InvVect) {
				for _, invVect := range invList {
					switch invVect.Type {
					case wire.InvTypeTx:
						utils.SafeSend(p.parentChannel, &PMMessage{
							Txid:   invVect.Hash.String(),
							Status: metamorph_api.Status_SEEN_ON_NETWORK,
						})
					}
				}
			}(invMsg.InvList)

		case wire.CmdGetData:
			dataMsg := msg.(*wire.MsgGetData)
			logger.Infof("Recv GETDATA (%d items)", len(dataMsg.InvList))
			for _, inv := range dataMsg.InvList {
				logger.Infof("        %s", inv.Hash.String())
			}
			p.handleGetDataMsg(dataMsg)

		case wire.CmdReject:
			rejMsg := msg.(*wire.MsgReject)
			utils.SafeSend(p.parentChannel, &PMMessage{
				Txid:   rejMsg.Hash.String(),
				Err:    fmt.Errorf("P2P rejection: %s", rejMsg.Reason),
				Status: metamorph_api.Status_REJECTED,
			})

		case wire.CmdVerAck:
			// Ignore

		default:
			logger.Warnf("Ignored %s", strings.ToUpper(msg.Command()))
		}
	}
}

func (p *Peer) handleGetDataMsg(dataMsg *wire.MsgGetData) {
	for _, invVect := range dataMsg.InvList {
		switch invVect.Type {
		case wire.InvTypeTx:
			logger.Infof("Request for TX: %s\n", invVect.Hash.String())

			txBytes, err := p.peerStore.GetTransactionBytes(invVect.Hash.CloneBytes())
			if err != nil {
				logger.Errorf("Unable to fetch tx %s from store: %v", invVect.Hash.String(), err)
				continue
			}

			if txBytes == nil {
				logger.Warnf("Unable to fetch tx %s from store: %v", invVect.Hash.String(), err)
				continue
			}

			tx, err := bsvutil.NewTxFromBytes(txBytes)
			if err != nil {
				log.Print(err) // Log and handle the error
				continue
			}

			p.writeChan <- tx.MsgTx()

		case wire.InvTypeBlock:
			logger.Infof("Request for Block: %s\n", invVect.Hash.String())

			if err := p.peerStore.ProcessBlock(invVect.Hash.CloneBytes()); err != nil {
				logger.Errorf("Unable to process block %s: %v", invVect.Hash.String(), err)
			}

		default:
			logger.Warnf("Unknown type: %d\n", invVect.Type)
		}
	}
}

func (p *Peer) writeChannelHandler() {
	for msg := range p.writeChan {
		if err := wire.WriteMessage(p.conn, msg, wire.ProtocolVersion, magic); err != nil {
			if errors.Is(err, io.EOF) {
				panic("WRITE EOF")
			}
			logger.Errorf("Failed to write message: %v", err)
		}

		if msg.Command() == wire.CmdTx {
			utils.SafeSend(p.parentChannel, &PMMessage{
				Txid:   msg.(*wire.MsgTx).TxHash().String(),
				Status: metamorph_api.Status_SENT_TO_NETWORK,
			})
		}

		switch m := msg.(type) {
		case *wire.MsgTx:
			logger.Infof("Sent TX: %s", m.TxHash().String())
		case *wire.MsgInv:
		default:
			logger.Infof("Sent %s", strings.ToUpper(msg.Command()))
		}
	}
}

func versionMessage() *wire.MsgVersion {
	lastBlock := int32(0)

	tcpAddrMe := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9999}
	me := wire.NewNetAddress(tcpAddrMe, wire.SFNodeNetwork)

	tcpAddrYou := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 18333}
	you := wire.NewNetAddress(tcpAddrYou, wire.SFNodeNetwork)

	nonce, err := wire.RandomUint64()
	if err != nil {
		logger.Infof("RandomUint64: error generating nonce: %v", err)
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
				logger.Errorf("Not sending ping to %s: %v", p, err)
				continue
			}
			p.writeChan <- wire.NewMsgPing(nonce)

		case <-p.quit:
			break out
		}
	}
}
