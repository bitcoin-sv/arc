package bcnet

import (
	"context"
	"log/slog"

	"github.com/libsv/go-p2p/wire"

	"github.com/bitcoin-sv/arc/internal/metamorph/bcnet/mcast"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/bitcoin-sv/arc/internal/p2p"
)

// Mediator acts as the central communication hub between metamorph processor and blockchain network,
// coordinating the interactions between the peer-to-peer messenger (p2p) and the multicast system (mcast).
// It is responsible for handling transactions and peer interactions depending on the operating mode (classic or hybrid).
//
// Fields:
// - `classic`: A flag indicating if the system is operating in classic mode (`true`) or hybrid mode (`false`).
// - `p2pMessenger`: The component responsible for managing peer-to-peer communications, including requesting and announcing transactions.
// - `mcaster`: The component responsible for sending transactions over multicast networks in hybrid mode.
//
// Methods:
// - `NewMediator`: Initializes a new `Mediator` with the specified logging, mode (classic or hybrid),
// p2p messenger, multicast system, and optional tracing attributes.
// - `AskForTxAsync`: Asynchronously requests a transaction by its hash from the network via P2P.
// - `AnnounceTxAsync`: Asynchronously announces a transaction to the network.
// In classic mode, it uses `p2pMessenger` to announce the transaction. In hybrid mode, it uses `mcaster` to send the transaction via multicast.
//
// Usage:
// - The `Mediator` abstracts the differences between classic (peer-to-peer) and hybrid (peer-to-peer and multicast) modes.
// - In classic mode, transactions are communicated directly with peers, whereas in hybrid mode, multicast groups are used for broader network communication.
// - Tracing functionality allows monitoring the flow of transactions and network requests, providing insights into system performance and behavior.
type Mediator struct {
	logger  *slog.Logger
	classic bool

	p2pMessenger *p2p.NetworkMessenger
	mcaster      *mcast.Multicaster
}

type Option func(*Mediator)

func NewMediator(l *slog.Logger, classic bool, messenger *p2p.NetworkMessenger, mcaster *mcast.Multicaster, opts ...Option) *Mediator {
	mode := "classic"
	if !classic {
		mode = "hybrid"
	}

	m := &Mediator{
		logger:       l.With("mode", mode),
		classic:      classic,
		p2pMessenger: messenger,
		mcaster:      mcaster,
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

func (m *Mediator) AskForTxAsync(_ context.Context, tx *store.Data) {
	m.p2pMessenger.RequestWithAutoBatch(tx.Hash, wire.InvTypeTx)
}

func (m *Mediator) AnnounceTxAsync(_ context.Context, tx *store.Data) {
	if m.classic {
		m.p2pMessenger.AnnounceWithAutoBatch(tx.Hash, wire.InvTypeTx)
	} else {
		_ = m.mcaster.SendTx(tx.RawTx)
	}
}

func (m *Mediator) GetPeers() []p2p.PeerI {
	return m.p2pMessenger.GetPeers()
}

func (m *Mediator) CountConnectedPeers() uint {
	return m.p2pMessenger.CountConnectedPeers()
}
