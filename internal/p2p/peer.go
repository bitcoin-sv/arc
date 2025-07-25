package p2p

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/libsv/go-p2p/wire"
)

const (
	defaultMaximumMessageSize = 32 * 1024 * 1024
	defaultReadBufferSize     = 4096
	connectionTimeoutDefault  = 30 * time.Second
	defaultPingInterval       = time.Minute
	defaultHealthTreshold     = 3 * time.Minute

	commandKey = "cmd"
	errKey     = "err"
)

var _ PeerI = (*Peer)(nil)

// Peer outgoing connection peer
type Peer struct {
	execWg        sync.WaitGroup
	execCtx       context.Context
	cancelExecCtx context.CancelFunc

	startMu   sync.Mutex
	connected atomic.Bool

	address          string
	network          wire.BitcoinNet
	servicesFlag     wire.ServiceFlag
	userAgentName    *string
	userAgentVersion *string
	dialer           Dialer

	connectionTimeout time.Duration
	lConn             net.Conn
	logger            *slog.Logger
	mh                MessageHandlerI

	writeCh      chan wire.Message
	nWriters     uint8
	maxMsgSize   int64
	readBuffSize int

	pingInterval    time.Duration
	healthThreshold time.Duration
	aliveCh         chan struct{}
	isUnhealthyCh   chan struct{}
}

type Dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

func NewPeer(logger *slog.Logger, msgHandler MessageHandlerI, address string, network wire.BitcoinNet, options ...PeerOptions) *Peer {
	p := &Peer{
		dialer: &net.Dialer{},
		logger: logger.With(
			slog.Group("peer",
				slog.String("network", network.String()),
				slog.String("address", address),
			),
		),
		mh: msgHandler,

		connectionTimeout: connectionTimeoutDefault,

		address:      address,
		network:      network,
		servicesFlag: wire.SFNodeNetwork,

		pingInterval:    defaultPingInterval,
		healthThreshold: defaultHealthTreshold,
		aliveCh:         make(chan struct{}, 10),
		isUnhealthyCh:   make(chan struct{}),

		maxMsgSize:   defaultMaximumMessageSize,
		readBuffSize: defaultReadBufferSize,
		nWriters:     1,
	}

	for _, opt := range options {
		opt(p)
	}

	if p.writeCh == nil {
		p.writeCh = make(chan wire.Message, 128)
	}

	return p
}

func (p *Peer) Connect() bool {
	p.startMu.Lock()
	defer p.startMu.Unlock()

	if p.connected.Load() {
		p.logger.Warn("Unexpected Connect() call. Peer is connected already.")
		return true
	}

	return p.connect()
}

func (p *Peer) Connected() bool {
	return p.connected.Load()
}

func (p *Peer) Restart() bool {
	p.startMu.Lock()
	defer p.startMu.Unlock()

	p.logger.Info("Restarting")
	if p.connected.Load() {
		p.disconnect()
	}

	return p.connect()
}

func (p *Peer) Shutdown() {
	p.startMu.Lock()
	defer p.startMu.Unlock()

	if !p.connected.Load() {
		return
	}

	p.logger.Info("Shutting down peer")
	p.disconnect()
	p.logger.Info("Shutdown peer complete")
}

func (p *Peer) IsUnhealthyCh() <-chan struct{} {
	return p.isUnhealthyCh
}
func (p *Peer) WriteMsg(msg wire.Message) {
	p.writeCh <- msg
}

func (p *Peer) Network() wire.BitcoinNet {
	return p.network
}

func (p *Peer) String() string {
	return p.address
}

func (p *Peer) connect() bool {
	p.logger.Info("Connecting")

	ctx := context.Background()
	execCtx, cancelFn := context.WithCancel(ctx)
	p.execCtx = execCtx
	p.cancelExecCtx = cancelFn

	defer p.healthMonitor()

	ctxDial, cancelDialFn := context.WithTimeout(ctx, p.connectionTimeout)
	defer cancelDialFn()

	lc, err := p.dialer.DialContext(ctxDial, "tcp", p.address)
	if err != nil {
		p.logger.Error("Failed to dial node", slog.String("err", err.Error()))
		return false
	}

	if ok := p.handshake(lc); !ok {
		_ = lc.Close()
		return false
	}

	p.lConn = lc
	p.listenForMessages()
	// run message writers
	for i := range p.nWriters {
		p.sendMessages(i)
	}

	p.keepAlive()

	p.connected.Store(true)
	p.logger.Info("Ready")

	return true
}

func (p *Peer) handshake(c net.Conn) (ok bool) {
	/* 1. send VER
	 * 2. wait for VERACK
	 * 3. wait for VER from node
	 * 4. send VERACK
	 */

	// send VerMsg
	me := wire.NewNetAddress(&net.TCPAddr{IP: nil, Port: 0}, p.servicesFlag) // shouldn't be mode configurable?

	nAddr, _ := net.ResolveTCPAddr("tcp", p.address) // address was validate already, we can omit error
	you := wire.NewNetAddress(nAddr, wire.SFNodeNetwork)

	nonce, err := wire.RandomUint64()
	if err != nil {
		p.logger.Warn("Handshake: failed to generate nonce, send VER with 0 nonce", slog.String(errKey, err.Error()))
	}

	const lastBlock = int32(0)
	verMsg := wire.NewMsgVersion(me, you, nonce, lastBlock)

	if p.userAgentName != nil && p.userAgentVersion != nil {
		err = verMsg.AddUserAgent(*p.userAgentName, *p.userAgentVersion)
		if err != nil {
			p.logger.Warn("Handshake: failed to add user agent, send VER without user agent", slog.String(errKey, err.Error()))
		}
	}

	err = wire.WriteMessage(c, verMsg, wire.ProtocolVersion, p.network)
	const handshakeFailed = "Handshake failed"
	if err != nil {
		p.logger.Error(handshakeFailed,
			slog.String("reason", "failed to write VER message"),
			slog.String(errKey, err.Error()),
		)

		return false
	}

	p.logger.Debug("Sent", slogUpperString(commandKey, verMsg.Command()))

	// wait for ACK, and VER from node send VERACK
	handshakeReadCtx, handshakeDoneFn := context.WithCancel(p.execCtx)
	defer handshakeDoneFn()

	read := make(chan readResult, 1)
	readController := make(chan struct{}, 1)

	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return

			case <-readController:
				msg, _, err := wire.ReadMessage(c, wire.ProtocolVersion, p.network)
				read <- readResult{msg, err}
			}
		}
	}(handshakeReadCtx)

	return p.performHandShake(c, read, readController, handshakeFailed)
	// if we exit the handshake loop, the handshake has completed successfully
}

func (p *Peer) performHandShake(c net.Conn, read chan readResult, readController chan struct{}, handshakeFailed string) bool {
	receivedVerAck := false
	sentVerAck := false
	var err error
	for {
		// "read" next message
		readController <- struct{}{}

		select {
		// peer was stopped
		case <-p.execCtx.Done():
			return false

		case <-time.After(1 * time.Minute):
			p.logger.Error(handshakeFailed, slog.String("reason", "handshake timeout"))
			return false

		case result := <-read:
			if result.err != nil {
				p.logger.Error(handshakeFailed, slog.String(errKey, result.err.Error()))
				return false
			}

			nmsg := result.msg

			switch nmsg.Command() {
			case wire.CmdVerAck:
				p.logger.Debug("Handshake: received VERACK")
				receivedVerAck = true

			case wire.CmdVersion:
				p.logger.Debug("Handshake: received VER")
				if sentVerAck {
					p.logger.Warn("Handshake: received version message after sending verack.")
					continue
				}

				// send VERACK to node
				ackMsg := wire.NewMsgVerAck()
				err = wire.WriteMessage(c, ackMsg, wire.ProtocolVersion, p.network)
				if err != nil {
					p.logger.Error(handshakeFailed,
						slog.String("reason", "failed to write VERACK message"),
						slog.String(errKey, err.Error()),
					)
					return false
				}

				p.logger.Debug("Handshake: sent VERACK")
				sentVerAck = true

			default:
				p.logger.Warn("Handshake: received unexpected message. Message was ignored", slogUpperString(commandKey, nmsg.Command()))
			}
		}
		if receivedVerAck && sentVerAck {
			break
		}
	}
	return true
}

func (p *Peer) keepAlive() {
	p.execWg.Add(1)

	go func() {
		p.logger.Debug("Start keep-alive")
		defer p.logger.Debug("Stop keep-alive")
		defer p.execWg.Done()

		t := time.NewTicker(p.pingInterval)
		defer t.Stop()

		for {
			select {
			case <-p.execCtx.Done():
				return
			case <-t.C:
				nonce, err := wire.RandomUint64()
				if err != nil {
					p.logger.Error("KeepAlive: failed to generate nonce for PING message", slog.String(errKey, err.Error()))
					continue
				}

				p.writeCh <- wire.NewMsgPing(nonce)
			}
		}
	}()
}

func (p *Peer) healthMonitor() {
	p.execWg.Add(1)

	go func() {
		p.logger.Debug("Start health monitor")
		defer p.logger.Debug("Stop health monitor")
		defer p.execWg.Done()

		// if no ping/pong signal is received for certain amount of time, mark peer as unhealthy and disconnect
		t := time.NewTicker(p.healthThreshold)
		defer t.Stop()

		for {
			select {
			case <-p.execCtx.Done():
				return

			case <-p.aliveCh:
				// ping-pong received so reset ticker
				t.Reset(p.healthThreshold)
				p.logger.Log(context.Background(), slogLvlTrace, "Connection is healthy - reset ticker", slog.Duration("interval", p.healthThreshold))

			case <-t.C:
				// no ping or pong for too long
				p.logger.Warn("Peer unhealthy - disconnecting")
				p.unhealthyDisconnect()
				return
			}
		}
	}()
}

func (p *Peer) disconnect() {
	p.logger.Info("Disconnecting")

	p.cancelExecCtx()
	p.execWg.Wait()

	if p.lConn != nil {
		_ = p.lConn.Close()
		p.lConn = nil
	}

	p.execCtx = nil
	p.cancelExecCtx = nil

	p.connected.Store(false)
	p.logger.Info("Disconnected")
}

func (p *Peer) unhealthyDisconnect() {
	go func() {
		// execute in new goroutine to avoid deadlock
		p.disconnect()

		select {
		case p.isUnhealthyCh <- struct{}{}:
		default: // Do not block if nothing is reading from channel
		}
	}()
}

func (p *Peer) listenForMessages() {
	p.execWg.Add(1)

	go func() {
		l := p.logger
		l.Debug("Starting read handler")
		defer l.Debug("Shutting down read handler")
		defer p.execWg.Done()

		reader := NewWireReaderSize(p.lConn, p.maxMsgSize, p.readBuffSize)
		for {
			msg, err := reader.ReadNextMsg(p.execCtx, wire.ProtocolVersion, p.network)
			if errors.Is(err, context.Canceled) {
				return
			}
			if err != nil {
				l.Error("Failed to read message", slog.String("err", err.Error()))
				// stop peer
				p.unhealthyDisconnect()
				return
			}

			cmd := msg.Command()
			l.Log(context.Background(), slogLvlTrace, "Received", slogUpperString(commandKey, cmd))

			switch cmd {
			// micro optimization - INV is the most frequently received message
			case wire.CmdInv:
				p.mh.OnReceive(msg, p)

			// ignore handshake type messages
			case wire.CmdVersion:
				fallthrough
			case wire.CmdVerAck:
				l.Warn("Received handshake message after handshake completed", slogUpperString(commandKey, cmd))

			// handle keep-alive ping-pong
			case wire.CmdPing:
				ping, ok := msg.(*wire.MsgPing)
				if !ok {
					p.logger.Warn("Received invalid PING")
					continue
				}

				p.aliveCh <- struct{}{}
				p.writeCh <- wire.NewMsgPong(ping.Nonce) // are we sure it should go with write channel not beside?

			case wire.CmdPong:
				p.aliveCh <- struct{}{}

			// pass message to client
			default:
				p.mh.OnReceive(msg, p)
			}
		}
	}()
}

func (p *Peer) sendMessages(n uint8) {
	p.execWg.Add(1)

	go func() {
		l := p.logger.With(slog.Int("instance", int(n)))

		l.Debug("Starting write handler")
		defer l.Debug("Shutting down write handler")
		defer p.execWg.Done()

		for {
			select {
			case <-p.execCtx.Done():
				return

			case msg := <-p.writeCh:
				// do not retry || TODO: rethink retry
				err := wire.WriteMessage(p.lConn, msg, wire.ProtocolVersion, p.network)
				if err != nil {
					l.Error("Failed to send message",
						slogUpperString(commandKey, msg.Command()),
						slog.String("err", err.Error()),
					)
					// stop peer
					p.unhealthyDisconnect()
					return
				}

				l.Log(context.Background(), slogLvlTrace, "Sent", slogUpperString(commandKey, msg.Command()))
				// let client react on sending msg
				p.mh.OnSend(msg, p)
			}
		}
	}()
}
