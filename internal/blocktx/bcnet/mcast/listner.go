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

var _ multicast.MessageHandlerI = (*Listner)(nil)

type Listner struct {
	hostname string

	logger    *slog.Logger
	store     store.BlocktxStore
	receiveCh chan<- *bcnet.BlockMessage

	blockGroup *multicast.Group[*bcnet.BlockMessage]
}

func NewMcastListner(l *slog.Logger, addr string, network wire.BitcoinNet, store store.BlocktxStore, receiveCh chan<- *bcnet.BlockMessage) *Listner {
	hostname, _ := os.Hostname()

	listner := Listner{
		logger:    l.With("module", "mcast-listner"),
		hostname:  hostname,
		store:     store,
		receiveCh: receiveCh,
	}

	listner.blockGroup = multicast.NewGroup[*bcnet.BlockMessage](l, &listner, addr, multicast.Read, network)
	return &listner
}

func (l *Listner) Connect() bool {
	return l.blockGroup.Connect()
}

func (l *Listner) Disconnect() {
	l.blockGroup.Disconnect()
}

// OnReceive should be fire & forget
func (l *Listner) OnReceive(msg wire.Message) {
	if msg.Command() == wire.CmdBlock {
		blockMsg, ok := msg.(*bcnet.BlockMessage)
		if !ok {
			l.logger.Error("Block msg receive", slog.Any("err", ErrUnableToCastWireMessage))
			return
		}

		// TODO: move it to mediator or smth
		// lock block for the current instance to process
		hash := blockMsg.Hash

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

// OnSend should be fire & forget
func (l *Listner) OnSend(_ wire.Message) {
	// ignore
}
