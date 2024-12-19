package multicast

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"

	"github.com/bitcoin-sv/arc/internal/p2p"
	"github.com/libsv/go-p2p/wire"
	"golang.org/x/net/ipv6"
)

type ModeFlag uint8

const (
	Read ModeFlag = 1 << iota
	Write
)

func (flag ModeFlag) Has(v ModeFlag) bool {
	return v&flag != 0
}

type MessageHandlerI interface {
	// OnReceive should be fire & forget
	OnReceive(msg wire.Message)
	// OnSend should be fire & forget
	OnSend(msg wire.Message)
}

type Group[T wire.Message] struct {
	execWg        sync.WaitGroup
	execCtx       context.Context
	cancelExecCtx context.CancelFunc

	startMu   sync.Mutex
	connected atomic.Bool

	addr string
	mode ModeFlag

	network      wire.BitcoinNet
	maxMsgSize   int64
	readBuffSize int

	mcastConn *ipv6ConnAdapter
	writeCh   chan T

	logger *slog.Logger
	mh     MessageHandlerI
}

func NewGroup[T wire.Message](l *slog.Logger, mh MessageHandlerI, addr string, mode ModeFlag, network wire.BitcoinNet /*TODO: add opts*/) *Group[T] {
	var tmp T
	l = l.With(
		slog.Group("mcast",
			slog.String("network", network.String()),
			slog.String("cmd", tmp.Command()),
			slog.String("address", addr),
		),
	)

	g := Group[T]{
		logger: l,
		mh:     mh,

		addr: addr,
		mode: mode,

		network:      network,
		maxMsgSize:   32 * 1024 * 1024,
		readBuffSize: 4096,
	}

	if mode.Has(Write) && g.writeCh == nil {
		g.writeCh = make(chan T, 256)
	}

	return &g
}

func (g *Group[T]) Connect() bool {
	g.startMu.Lock()
	defer g.startMu.Unlock()

	if g.connected.Load() {
		g.logger.Warn("Unexpected Connect() call. Group is connected already.")
		return true
	}

	return g.connect()
}

func (g *Group[T]) Disconnect() {
	g.startMu.Lock()
	defer g.startMu.Unlock()

	if !g.connected.Load() {
		return
	}

	g.disconnect()
}

func (g *Group[T]) WriteMsg(msg T) {
	if !g.mode.Has(Write) {
		panic("Cannot write to group in read-only mode")
	}

	g.writeCh <- msg
}

func (g *Group[T]) connect() bool {
	g.logger.Info("Connecting")

	ctx, cancelFn := context.WithCancel(context.Background())
	g.execCtx = ctx
	g.cancelExecCtx = cancelFn

	udpAddr, err := net.ResolveUDPAddr("udp6", g.addr)
	if err != nil {
		g.logger.Error("Cannot resolve UDP address", slog.String("err", err.Error()))
		return false
	}

	conn, err := net.ListenUDP("udp6", udpAddr)
	if err != nil {
		g.logger.Error("Failed to dial node", slog.String("err", err.Error()))
		return false
	}

	pConn := ipv6.NewPacketConn(conn)
	g.mcastConn = &ipv6ConnAdapter{Conn: pConn}

	if g.mode.Has(Read) {
		err = pConn.JoinGroup(nil, udpAddr) // TODO: define net interface
		if err != nil {
			g.logger.Error("Failed to join mcast group", slog.String("err", err.Error()))
			return false
		}

		g.listenForMessages()
	}

	if g.mode.Has(Write) {
		g.sendMessages()
	}

	g.connected.Store(true)
	g.logger.Info("Ready")
	return true
}

func (g *Group[T]) disconnect() {
	g.logger.Info("Disconnecting")

	g.cancelExecCtx()
	g.execWg.Wait()

	if g.mode.Has(Read) {
		udpAddr, _ := net.ResolveUDPAddr("udp6", g.addr)
		_ = g.mcastConn.Conn.LeaveGroup(nil, udpAddr) // TODO: define net interface
	}

	_ = g.mcastConn.Conn.Close()
	g.mcastConn = nil
	g.execCtx = nil
	g.cancelExecCtx = nil

	g.connected.Store(false)
	g.logger.Info("Disconnected")
}

func (g *Group[T]) listenForMessages() {
	g.execWg.Add(1)

	go func() {
		g.logger.Debug("Start listen handler")
		defer g.logger.Debug("Stop listen handler")
		defer g.execWg.Done()

		var tmp T
		expectedCmd := tmp.Command()

		reader := p2p.NewWireReaderSize(g.mcastConn, g.maxMsgSize, g.readBuffSize)
		for {
			msg, err := reader.ReadNextMsg(g.execCtx, wire.ProtocolVersion, g.network)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					g.logger.Debug("Stop listen handler")
					return
				}

				// TODO: think how to handle read errors

				g.logger.Error("Read failed", slog.String("err", err.Error()))
				// stop group
				//p.unhealthyDisconnect() -- auto reconnect?
				return
			}

			cmd := msg.Command()
			if cmd != expectedCmd {
				g.logger.Warn("Unexpected message type from mcast group. Message ignored", slog.String("cmd", cmd))
				continue
			}

			g.logger.Log(context.Background(), slogLvlTrace, "Received message")
			g.mh.OnReceive(msg)
		}
	}()
}

func (g *Group[T]) sendMessages() {
	g.execWg.Add(1)

	go func() {
		g.logger.Debug("Start send handler")
		defer g.execWg.Done()

		for {
			select {
			case <-g.execCtx.Done():
				g.logger.Debug("Stop send handler")
				return

			case msg := <-g.writeCh:
				// do not retry
				err := wire.WriteMessage(g.mcastConn, msg, wire.ProtocolVersion, g.network)
				if err != nil {
					// TODO: think how to handle send errors
					g.logger.Error("Failed to send message", slog.String("err", err.Error()))

					// // stop group
					// p.unhealthyDisconnect()
					return
				}

				g.logger.Log(context.Background(), slogLvlTrace, "Sent message")
				// let client react on sending msg
				g.mh.OnSend(msg)
			}
		}
	}()
}

const slogLvlTrace slog.Level = slog.LevelDebug - 4
