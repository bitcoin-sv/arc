package main

/*
Description:
This application serves as a sidecar for a Bitcoin SV node, acting as a bridge between the P2P network and multicast groups.
It facilitates communication between these two networks, enabling message exchange in both directions: from P2P to multicast and from multicast to P2P.

It handles two main types of messages:
1. Block and transaction messages (MsgBlock, MsgTx) transmitted over both P2P and multicast networks.
2. Data requests (e.g., MsgGetData, MsgInv) sent in response to incoming requests on the P2P network.

The application is configured via a configuration file that specifies:
- P2P node address.
- Multicast group addresses.

The app:
1. Connects to specified node via P2P.
2. Joins the appropriate multicast groups.
3. Handles sending and receiving messages between P2P and multicast.
*/

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/libsv/go-p2p/wire"

	"github.com/bitcoin-sv/arc/config"
	arcLogger "github.com/bitcoin-sv/arc/internal/logger"
	"github.com/bitcoin-sv/arc/internal/multicast"
	"github.com/bitcoin-sv/arc/internal/p2p"
	"github.com/bitcoin-sv/arc/internal/version"
)

func main() {
	err := run()
	if err != nil {
		log.Fatalf("Failed to run multicast emulator: %v", err)
	}

	return
}

func run() error {
	configDir, _, _, _, _, _, _ := parseFlags()
	arcConfig, err := config.Load(configDir)
	if err != nil {
		return err
	}

	logger, err := arcLogger.NewLogger(arcConfig.LogLevel, arcConfig.LogFormat)
	if err != nil {
		return fmt.Errorf("failed to create logger: %v", err)
	}

	stopFn, err := startMcastSideCar(logger, arcConfig)
	if err != nil {
		return fmt.Errorf("failed to start Multicast-P2P bridge: %v", err)
	}

	// wait for termination signal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM, syscall.SIGINT)

	<-signalChan

	stopFn()
	return nil
}

func startMcastSideCar(logger *slog.Logger, arcConfig *config.ArcConfig) (func(), error) {
	logger.Info("Starting Multicast Emulator sidecar", slog.String("version", version.Version), slog.String("commit", version.Commit))

	network, err := config.GetNetwork(arcConfig.Blocktx.BlockchainNetwork.Network)
	if err != nil {
		return nil, err
	}

	communicationBridge := &MulticastP2PBridge{
		logger: logger.With(slog.String("module", "multicast-p2p-bridge")),
	}

	// connect to peer
	peerCfg := arcConfig.Blocktx.BlockchainNetwork.Peers[0]
	peer, err := connectPeer(logger, peerCfg, communicationBridge, network)
	if err != nil {
		return nil, err
	}
	communicationBridge.peer = peer

	// connect to mcast groups
	blockGroupCfg := arcConfig.Blocktx.BlockchainNetwork.Mcast.McastBlock
	blockGroup, err := connectMcastGroup[*wire.MsgBlock](logger, &blockGroupCfg, communicationBridge, network)
	if err != nil {
		return nil, err
	}
	communicationBridge.blockGroup = blockGroup

	txGroupCfg := arcConfig.Metamorph.BlockchainNetwork.Mcast.McastTx
	txGroup, err := connectMcastGroup[*wire.MsgTx](logger, &txGroupCfg, communicationBridge, network)
	if err != nil {
		return nil, err
	}
	communicationBridge.txGroup = txGroup

	// return cleanup function
	return func() {
		peer.Shutdown()
		blockGroup.Disconnect()
		txGroup.Disconnect()
	}, nil
}

func connectPeer(logger *slog.Logger, peerCfg *config.PeerConfig, msgHandler p2p.MessageHandlerI, network wire.BitcoinNet) (*p2p.Peer, error) {
	peerURL, err := peerCfg.GetP2PUrl()
	if err != nil {
		return nil, err
	}

	peer := p2p.NewPeer(logger, msgHandler, peerURL, network)
	connected := peer.Connect()
	if !connected {
		return nil, errors.New("cannot connect to peer")
	}

	return peer, nil
}

func connectMcastGroup[T wire.Message](logger *slog.Logger, grCfg *config.McastGroup, mcastMsgHandler multicast.MessageHandlerI, network wire.BitcoinNet) (*multicast.Group[T], error) {
	mcastGroup := multicast.NewGroup[T](logger, mcastMsgHandler, grCfg.Address, multicast.Read|multicast.Write, network)
	connected := mcastGroup.Connect()
	if !connected {
		return nil, fmt.Errorf("cannot connect to %s multicast group", grCfg.Address)
	}

	return mcastGroup, nil
}

func parseFlags() (string, bool, bool, bool, bool, bool, string) {
	startAPI := flag.Bool("api", false, "Start ARC API server")
	startMetamorph := flag.Bool("metamorph", false, "Start Metamorph")
	startBlockTx := flag.Bool("blocktx", false, "Start BlockTx")
	startK8sWatcher := flag.Bool("k8s-watcher", false, "Start K8s-Watcher")
	startCallbacker := flag.Bool("callbacker", false, "Start Callbacker")
	help := flag.Bool("help", false, "Show help")
	dumpConfigFile := flag.String("dump_config", "", "Dump config to specified file and exit")
	configDir := flag.String("config", "", "Path to configuration file")

	flag.Parse()

	if *help {
		fmt.Println("Usage: main [options]")
		return
	}

	return *configDir, *startAPI, *startMetamorph, *startBlockTx, *startK8sWatcher, *startCallbacker, *dumpConfigFile
}

// MulticastP2PBridge is a bridge between a P2P connection and a multicast groups.
// It facilitates the handling of messages between P2P and multicast communication channels.
// Specifically, it listens for messages from both channels and translates or forwards them accordingly.
type MulticastP2PBridge struct {
	logger *slog.Logger

	blockGroup *multicast.Group[*wire.MsgBlock]
	txGroup    *multicast.Group[*wire.MsgTx]
	peer       *p2p.Peer

	txCache sync.Map
}

var (
	_ multicast.MessageHandlerI = (*MulticastP2PBridge)(nil)
	_ p2p.MessageHandlerI       = (*MulticastP2PBridge)(nil)
)

// OnReceive handles incoming messages depending on command type. Implements p2p.MessageHandlerI
func (b *MulticastP2PBridge) OnReceive(msg wire.Message, peer p2p.PeerI) {
	cmd := msg.Command()
	switch cmd {
	case wire.CmdInv:
		invMsg, ok := msg.(*wire.MsgInv)
		if !ok {
			return
		}
		b.handleReceivedP2pInvMsg(invMsg, peer)

	case wire.CmdBlock:
		blockMsg, ok := msg.(*wire.MsgBlock)
		if !ok {
			b.logger.Error("cannot cast msg to wire.MsgBlock")
			return
		}

		b.handleReceivedBlockMsg(blockMsg, peer)

	case wire.CmdGetData:
		getMsg, ok := msg.(*wire.MsgGetData)
		if !ok {
			b.logger.Error("cannot cast msg to wire.MsgGetData")
			return
		}

		b.handleReceivedGetDataMsg(getMsg, peer)

	default:
		// ignore other msgs
	}
}

// OnSend handles outgoing messages depending on command type
func (b *MulticastP2PBridge) OnSend(_ wire.Message, _ p2p.PeerI) {
	// ignore
}

func (b *MulticastP2PBridge) handleReceivedP2pInvMsg(msg *wire.MsgInv, peer p2p.PeerI) {
	for _, inv := range msg.InvList {
		if inv.Type == wire.InvTypeBlock {
			b.logger.Info("Received BlockINV", slog.String("hash", inv.Hash.String()), slog.String("peer", peer.String()))

			b.logger.Info("Request Block from peer", slog.String("hash", inv.Hash.String()), slog.String("peer", peer.String()))
			msg := wire.NewMsgGetDataSizeHint(1)
			_ = msg.AddInvVect(wire.NewInvVect(wire.InvTypeBlock, &inv.Hash)) // ignore error at this point
			peer.WriteMsg(msg)
		}

		// ignore other inv
	}
}

func (b *MulticastP2PBridge) handleReceivedBlockMsg(blockMsg *wire.MsgBlock, peer p2p.PeerI) {
	if b.blockGroup == nil {
		b.logger.Warn("multicast is not ready yet")
		return
	}

	b.logger.Info("Received BlockMsg", slog.String("hash", blockMsg.BlockHash().String()), slog.String("peer", peer.String()))
	b.logger.Info("Send BlockMsg to multicast handler", slog.String("hash", blockMsg.BlockHash().String()), slog.String("peer", peer.String()))
	b.blockGroup.WriteMsg(blockMsg)
}

func (b *MulticastP2PBridge) handleReceivedGetDataMsg(getMsg *wire.MsgGetData, peer p2p.PeerI) {
	b.logger.Info("Peer requested data", slog.String("peer", peer.String()))
	for _, inv := range getMsg.InvList {
		if inv.Type == wire.InvTypeTx {
			// check if tx is in memory and send it to peer
			anyMsg, found := b.txCache.Load(inv.Hash)
			if found {
				txMsg := anyMsg.(*wire.MsgTx) // nolint:errcheck,revive
				peer.WriteMsg(txMsg)
				b.logger.Info("Sent requested data to the peer", slog.String("hash", inv.Hash.String()), slog.String("peer", peer.String()))
			}
		}
		// ignore other inv
	}
}

// OnReceiveFromMcast handles incoming messages from multicast group depending on command type. Implements multicast.MessageHandlerI
func (b *MulticastP2PBridge) OnReceiveFromMcast(msg wire.Message) {
	cmd := msg.Command()
	switch cmd {
	case wire.CmdBlock:
		blockmsg := msg.(*wire.MsgBlock) // nolint:errcheck,revive
		b.logger.Info("Received BlockMsg from multicast", slog.String("hash", blockmsg.BlockHash().String()))

	case wire.CmdTx:
		txmsg := msg.(*wire.MsgTx) // nolint:errcheck,revive
		b.handleReceivedMcastTxMsg(txmsg)

	default:
		b.logger.Error("Unexpected msg from multicast group!", slog.String("unexpected.cmd", cmd))
	}
}

// OnSendToMcast handles outgoing messages to multicast group depending on command type
func (b *MulticastP2PBridge) OnSendToMcast(msg wire.Message) {
	cmd := msg.Command()
	switch cmd {
	case wire.CmdBlock:
		blockmsg := msg.(*wire.MsgBlock) // nolint:errcheck,revive
		b.logger.Info("Sent BlockMsg to multicast", slog.String("hash", blockmsg.BlockHash().String()))

	case wire.CmdTx:
		txmsg := msg.(*wire.MsgTx) // nolint:errcheck,revive
		b.logger.Info("Sent TxMsg to multicast", slog.String("hash", txmsg.TxHash().String()))

	default:
		b.logger.Error("Unexpected msg sent to multicast group!", slog.String("unexpected.cmd", cmd))
	}
}

func (b *MulticastP2PBridge) handleReceivedMcastTxMsg(txmsg *wire.MsgTx) {
	txHash := txmsg.TxHash()
	b.logger.Info("Received TxMsg from multicast", slog.String("hash", txHash.String()))

	// save TxMsg for later to send it to peer when it request it
	b.txCache.Store(txHash, txmsg)

	// announce tx to peer
	b.logger.Info("Send INV to peer", slog.String("hash", txHash.String()), slog.String("peer", b.peer.String()))

	invMsg := wire.NewMsgInv()
	_ = invMsg.AddInvVect(wire.NewInvVect(wire.InvTypeTx, &txHash))
	b.peer.WriteMsg(invMsg)
}
