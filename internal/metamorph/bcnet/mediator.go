package bcnet

import (
	"context"
	"log/slog"
	"runtime"

	"github.com/bitcoin-sv/arc/internal/metamorph/bcnet/mcast"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/bitcoin-sv/arc/internal/p2p"
	"github.com/bitcoin-sv/arc/internal/tracing"
	"github.com/libsv/go-p2p/wire"
	"go.opentelemetry.io/otel/attribute"
)

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

type Mediator struct {
	logger  *slog.Logger
	classic bool

	p2pMessenger *p2p.NetworkMessenger
	mcaster      *mcast.Multicaster

	tracingEnabled    bool
	tracingAttributes []attribute.KeyValue
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

func (m *Mediator) AskForTxAsync(ctx context.Context, tx *store.Data) {
	_, span := tracing.StartTracing(ctx, "AskForTxAsync", m.tracingEnabled, m.tracingAttributes...)

	m.p2pMessenger.RequestWithAutoBatch(tx.Hash, wire.InvTypeTx)
	tracing.EndTracing(span, nil)
}

func (m *Mediator) AnnounceTxAsync(ctx context.Context, tx *store.Data) {
	_, span := tracing.StartTracing(ctx, "AskForTxAsync", m.tracingEnabled, m.tracingAttributes...)

	if m.classic {
		m.p2pMessenger.AnnounceWithAutoBatch(tx.Hash, wire.InvTypeTx)
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
