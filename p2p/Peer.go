package p2p

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"
	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/p2p/blockchain"
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
	HandleBlockAnnouncement(hash []byte, peer PeerI) error
	InsertBlock(blockHash []byte, merkleRootHash []byte, previousBlockHash []byte, height uint64, peer PeerI) (uint64, error)
	MarkTransactionsAsMined(blockId uint64, txHashes [][]byte) error
	MarkBlockAsProcessed(*blocktx_api.Block) error
}

type Peer struct {
	address       string
	conn          net.Conn
	peerStore     PeerStoreI
	parentChannel chan *PMMessage
	writeChan     chan wire.Message
	quit          chan struct{}
	initialized   sync.WaitGroup
}

func NewPeer(address string, peerStore PeerStoreI) (*Peer, error) {
	writeChan := make(chan wire.Message, 100)

	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("could not dial node [%s]: %v", address, err)
	}

	p := &Peer{
		conn:        conn,
		address:     address,
		writeChan:   writeChan,
		peerStore:   peerStore,
		initialized: sync.WaitGroup{},
	}

	// create the init lock for Version and VerAck
	p.initialized.Add(2)

	go p.readHandler()
	go p.pingHandler()
	go p.writeChannelHandler()

	// write version message to our peer directly and not through the write channel,
	// write channel is not ready to send message until the VERACK handshake is done
	msg := versionMessage(address)

	if err = wire.WriteMessage(p.conn, msg, wire.ProtocolVersion, magic); err != nil {
		return nil, fmt.Errorf("failed to write message: %v", err)
	}
	p.LogInfo("Sent %s", strings.ToUpper(msg.Command()))

	return p, nil
}

func (p *Peer) AddParentMessageChannel(parentChannel chan *PMMessage) PeerI {
	p.parentChannel = parentChannel

	return p
}

func (p *Peer) WriteMsg(msg wire.Message) {
	p.writeChan <- msg
}

func (p *Peer) String() string {
	return p.address
}

func (p *Peer) LogError(format string, args ...interface{}) {
	logger.Errorf("[%s] "+format, append([]interface{}{p.address}, args...)...)
}

func (p *Peer) LogInfo(format string, args ...interface{}) {
	logger.Infof("[%s] "+format, append([]interface{}{p.address}, args...)...)
}

func (p *Peer) readHandler() {
	for {
		msg, b, err := wire.ReadMessage(p.conn, wire.ProtocolVersion, magic)
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
			if err = wire.WriteMessage(p.conn, verackMsg, wire.ProtocolVersion, magic); err != nil {
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
						utils.SafeSend(p.parentChannel, &PMMessage{
							Txid:   invVect.Hash.String(),
							Status: metamorph_api.Status_SEEN_ON_NETWORK,
						})
					case wire.InvTypeBlock:
						if err = p.peerStore.HandleBlockAnnouncement(invVect.Hash.CloneBytes(), p); err != nil {
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

			blockHash := blockMsg.BlockHash()
			blockHashBytes := blockHash.CloneBytes()

			txs := make([][]byte, blockMsg.TxCount)

			coinbaseTx := wire.NewMsgTx(1)
			if err := coinbaseTx.Bsvdecode(blockMsg.TransactionReader, blockMsg.ProtocolVersion, blockMsg.MessageEncoding); err != nil {
				p.LogError("Unable to read transaction from block %s: %v", blockHash.String(), err)
				continue
			}
			txHash := coinbaseTx.TxHash()
			// coinbase tx is always the first tx in the block
			txs[0] = txHash.CloneBytes()

			height := extractHeightFromCoinbaseTx(coinbaseTx)

			previousBlockHash := blockMsg.Header.PrevBlock
			previousBlockHashBytes := previousBlockHash.CloneBytes()

			merkleRoot := blockMsg.Header.MerkleRoot
			merkleRootBytes := merkleRoot.CloneBytes()

			blockId, err := p.peerStore.InsertBlock(blockHashBytes, merkleRootBytes, previousBlockHashBytes, height, p)
			if err != nil {
				p.LogError("Unable to insert block %s: %v", blockHash.String(), err)
				continue
			}

			for i := uint64(1); i < blockMsg.TxCount; i++ {
				tx := wire.NewMsgTx(1)
				if err := tx.Bsvdecode(blockMsg.TransactionReader, blockMsg.ProtocolVersion, blockMsg.MessageEncoding); err != nil {
					p.LogError("Unable to read transaction from block %s: %v", blockHash.String(), err)
					continue
				}
				txHash = tx.TxHash()
				txs[i] = txHash.CloneBytes()
			}

			if err := p.peerStore.MarkTransactionsAsMined(blockId, txs); err != nil {
				p.LogError("Unable to mark block as mined %s: %v", blockHash.String(), err)
				continue
			}

			calculatedMerkleRoot := blockchain.BuildMerkleTreeStore(txs)
			if !bytes.Equal(calculatedMerkleRoot[len(calculatedMerkleRoot)-1], merkleRootBytes) {
				p.LogError("Merkle root mismatch for block %s", blockHash.String())
				continue
			}

			block := &blocktx_api.Block{
				Hash:         blockHashBytes,
				MerkleRoot:   merkleRootBytes,
				PreviousHash: previousBlockHashBytes,
				Height:       height,
			}

			if err := p.peerStore.MarkBlockAsProcessed(block); err != nil {
				p.LogError("Unable to mark block as processed %s: %v", blockHash.String(), err)
				continue
			}

		case wire.CmdReject:
			rejMsg := msg.(*wire.MsgReject)
			utils.SafeSend(p.parentChannel, &PMMessage{
				Txid:   rejMsg.Hash.String(),
				Err:    fmt.Errorf("P2P rejection: %s", rejMsg.Reason),
				Status: metamorph_api.Status_REJECTED,
			})

		case wire.CmdVerAck:
			p.LogInfo("Recv %s", strings.ToUpper(msg.Command()))
			p.initialized.Done()

		default:
			logger.Warnf("Ignored %s", strings.ToUpper(msg.Command()))
		}
	}
}

func (p *Peer) handleGetDataMsg(dataMsg *wire.MsgGetData) {
	for _, invVect := range dataMsg.InvList {
		switch invVect.Type {
		case wire.InvTypeTx:
			p.LogInfo("Request for TX: %s\n", invVect.Hash.String())

			txBytes, err := p.peerStore.GetTransactionBytes(invVect.Hash.CloneBytes())
			if err != nil {
				p.LogError("Unable to fetch tx %s from store: %v", invVect.Hash.String(), err)
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
			p.LogInfo("Request for Block: %s\n", invVect.Hash.String())

		default:
			logger.Warnf("Unknown type: %d\n", invVect.Type)
		}
	}
}

func (p *Peer) writeChannelHandler() {
	p.initialized.Wait() // wait to send new messages until we are initialized

	for msg := range p.writeChan {
		if err := wire.WriteMessage(p.conn, msg, wire.ProtocolVersion, magic); err != nil {
			if errors.Is(err, io.EOF) {
				panic("WRITE EOF")
			}
			p.LogError("Failed to write message: %v", err)
		}

		if msg.Command() == wire.CmdTx {
			utils.SafeSend(p.parentChannel, &PMMessage{
				Txid:   msg.(*wire.MsgTx).TxHash().String(),
				Status: metamorph_api.Status_SENT_TO_NETWORK,
			})
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

func versionMessage(address string) *wire.MsgVersion {
	lastBlock := int32(0)

	tcpAddrMe := &net.TCPAddr{IP: nil, Port: 0}
	me := wire.NewNetAddress(tcpAddrMe, wire.SFNodeNetwork)

	parts := strings.Split(address, ":")
	if len(parts) != 2 {
		panic(fmt.Sprintf("Could not parse address %s", address))
	}

	p, err := strconv.Atoi(parts[1])
	if err != nil {
		panic(fmt.Sprintf("Could not parse port %s", parts[1]))
	}

	tcpAddrYou := &net.TCPAddr{IP: net.ParseIP(parts[0]), Port: p}
	you := wire.NewNetAddress(tcpAddrYou, wire.SFNodeNetwork)

	nonce, err := wire.RandomUint64()
	if err != nil {
		logger.Errorf("RandomUint64: error generating nonce: %v", err)
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

func extractHeightFromCoinbaseTx(tx *wire.MsgTx) uint64 {
	// Coinbase tx has a special format, the height is encoded in the first 4 bytes of the scriptSig
	// https://en.bitcoin.it/wiki/Protocol_documentation#tx
	// Get the length
	length := int(tx.TxIn[0].SignatureScript[0])

	if len(tx.TxIn[0].SignatureScript) < length+1 {
		return 0
	}

	b := make([]byte, 8)

	for i := 0; i < length; i++ {
		b[i] = tx.TxIn[0].SignatureScript[i+1]
	}

	return binary.LittleEndian.Uint64(b)
}
