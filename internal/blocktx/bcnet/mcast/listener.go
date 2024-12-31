package mcast

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"github.com/bitcoin-sv/arc/internal/blocktx/bcnet"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/internal/multicast"
	"github.com/libsv/go-p2p/wire"
)

var ErrUnableToCastWireMessage = errors.New("unable to cast wire.Message to blockchain.BlockMessage")

var _ multicast.MessageHandlerI = (*Listener)(nil)

// Listener is a multicast message listener specifically designed for processing blocks messages.
//
// Responsibilities:
// - Connects to a multicast group and listens for block messages (`CmdBlock`).
// - Processes incoming block messages by marking them for processing in the storage to ensure that only one instance processes a block at a time.
// - Ignores non-block messages to optimize processing and maintain focus on relevant data.
//
// Key Methods:
// - `NewMcastListener`: Initializes a new Listener instance, setting up the multicast group for reading block messages.
// - `Connect`: Establishes the connection to the multicast group.
// - `Disconnect`: Leaves the multicast group.
// - `OnReceive`: Handles received multicast messages, verifying their type and ensuring proper processing of block messages.
// - `OnSend`: Placeholder for handling messages sent to the multicast group, currently ignored.
//
// Behavior:
// - On receiving a block message (`CmdBlock`):
//   1. Verifies the message type.
//   2. Tries to mark the block as being processed by the current instance.
//   3. Forwards the block message to the `receiveCh` for further handling.
// - Logs and gracefully handles errors in block processing, ensuring robustness in a distributed system.

type Listener struct {
	hostname string

	logger    *slog.Logger
	store     store.BlocktxStore
	receiveCh chan<- *bcnet.BlockMessage

	blockGroup *multicast.Group[*bcnet.BlockMessage]
}

func NewMcastListener(l *slog.Logger, addr string, network wire.BitcoinNet, store store.BlocktxStore, receiveCh chan<- *bcnet.BlockMessage) *Listener {
	hostname, _ := os.Hostname()

	listner := Listener{
		logger:    l.With("module", "mcast-listener"),
		hostname:  hostname,
		store:     store,
		receiveCh: receiveCh,
	}

	listner.blockGroup = multicast.NewGroup[*bcnet.BlockMessage](l, &listner, addr, multicast.Read, network)
	return &listner
}

func (l *Listener) Connect() bool {
	return l.blockGroup.Connect()
}

func (l *Listener) Disconnect() {
	l.blockGroup.Disconnect()
}

// OnReceiveFromMcast handles received messages from multicast group
func (l *Listener) OnReceiveFromMcast(msg wire.Message) {
	if msg.Command() == wire.CmdBlock {
		blockMsg, ok := msg.(*bcnet.BlockMessage)
		if !ok {
			l.logger.Error("Block msg receive", slog.Any("err", ErrUnableToCastWireMessage))
			return
		}

		// TODO: move it to mediator or smth
		// lock block for the current instance to process
		hash := blockMsg.Hash

		l.logger.Info("Received BLOCK msg from multicast group", slog.String("hash", hash.String()))

		processedBy, err := l.store.SetBlockProcessing(context.Background(), hash, l.hostname)
		if err != nil {
			// block is already being processed by another blocktx instance
			if errors.Is(err, store.ErrBlockProcessingDuplicateKey) {
				l.logger.Debug("block processing already in progress", slog.String("hash", hash.String()), slog.String("processed_by", processedBy))
				return
			}

			l.logger.Error("failed to set block processing", slog.String("hash", hash.String()), slog.String("err", err.Error()))
			return
		}

		// p.startBlockProcessGuard(p.ctx, hash) // handle it somehow

		l.receiveCh <- blockMsg
	}

	// ignore other messages
}

// OnSendToMcast handles sent messages to multicast group
func (l *Listener) OnSendToMcast(_ wire.Message) {
	// ignore
}
