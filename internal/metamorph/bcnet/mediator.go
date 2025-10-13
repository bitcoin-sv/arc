package bcnet

import (
	"context"
	"log/slog"
	"runtime"

	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bitcoin-sv/arc/internal/global"
	"github.com/bitcoin-sv/arc/internal/metamorph/bcnet/mcast"
	"github.com/bitcoin-sv/arc/internal/p2p"
	"github.com/bitcoin-sv/arc/pkg/tracing"
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

	tracingEnabled    bool
	tracingAttributes []attribute.KeyValue
}

type Option func(*Mediator)

func WithTracer(attr ...attribute.KeyValue) Option {
	return func(p *Mediator) {
		p.tracingEnabled = true
		if len(attr) > 0 {
			p.tracingAttributes = append(p.tracingAttributes, attr...)
		}
		_, file, _, ok := runtime.Caller(1)
		if ok {
			p.tracingAttributes = append(p.tracingAttributes, attribute.String("file", file))
		}
	}
}

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

func (m *Mediator) AskForTxAsync(ctx context.Context, tx *global.Data) {
	_, span := tracing.StartTracing(ctx, "AskForTxAsync", m.tracingEnabled, m.tracingAttributes...)

	hash, _ := chainhash.NewHash(tx.Hash.CloneBytes())
	m.p2pMessenger.RequestWithAutoBatch(hash, wire.InvTypeTx)
	tracing.EndTracing(span, nil)
}

func (m *Mediator) AnnounceTxAsync(ctx context.Context, tx *global.Data) {
	_, span := tracing.StartTracing(ctx, "AskForTxAsync", m.tracingEnabled, m.tracingAttributes...)

	if m.classic {
		hash, _ := chainhash.NewHash(tx.Hash.CloneBytes())
		m.p2pMessenger.AnnounceWithAutoBatch(hash, wire.InvTypeTx)
	} else {
		_ = m.mcaster.SendTx(tx.RawTx)
	}

	tracing.EndTracing(span, nil)
}

func (m *Mediator) GetPeers() []p2p.PeerI {
	return m.p2pMessenger.GetPeers()
}

func (m *Mediator) CountConnectedPeers() uint {
	return m.p2pMessenger.CountConnectedPeers()
}
