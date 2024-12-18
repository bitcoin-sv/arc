package p2p

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/libsv/go-p2p/wire"
)

const (
	defaultMaximumMessageSize = 32 * 1024 * 1024

	defaultPingInterval   = 2 * time.Minute
	defaultHealthTreshold = 3 * time.Minute

	commandKey = "cmd"
	errKey     = "err"
)

var _ PeerI = (*Peer)(nil)

// outgoing connection peer
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
	dial             func(network, address string) (net.Conn, error)

	lConn net.Conn

	l  *slog.Logger
	mh MessageHandlerI

	writeCh    chan wire.Message
	nWriters   uint8
	maxMsgSize int64

	pingInterval    time.Duration
	healthThreshold time.Duration
	aliveCh         chan struct{}
	isUnhealthyCh   chan struct{}
}

func NewPeer(logger *slog.Logger, msgHandler MessageHandlerI, address string, network wire.BitcoinNet, options ...PeerOptions) *Peer {
	l := logger.With(
		slog.Group("peer",
			slog.String("network", network.String()),
			slog.String("address", address),
		),
	)

	p := &Peer{
		dial: net.Dial,
		l:    l,
		mh:   msgHandler,

		address:      address,
		network:      network,
		servicesFlag: wire.SFNodeNetwork,

		pingInterval:    defaultPingInterval,
		healthThreshold: defaultHealthTreshold,
		aliveCh:         make(chan struct{}, 1),
		isUnhealthyCh:   make(chan struct{}),

		maxMsgSize: defaultMaximumMessageSize,
		nWriters:   1,
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
	if p.connected.Load() {
		p.l.Warn("Unexpected Connect() call. Peer is connected already.")
		return true
	}

	p.startMu.Lock()
	defer p.startMu.Unlock()

	return p.connect()
}

func (p *Peer) Connected() bool {
	return p.connected.Load()
}

func (p *Peer) Restart() bool {
	p.startMu.Lock()
	defer p.startMu.Unlock()

	p.l.Info("Restarting")
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

	p.l.Info("Shutting down")
	p.disconnect()
	p.l.Info("Shutdown complete")
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
	p.l.Info("Connecting")

	ctx, cancelFn := context.WithCancel(context.Background())
	p.execCtx = ctx
	p.cancelExecCtx = cancelFn

	lc, err := p.dial("tcp", p.address)
	if err != nil {
		p.l.Error("Failed to dial node",
			slog.String("additional-info", err.Error()),
		)
		return false
	}

	if ok := p.handshake(lc); !ok {
		_ = lc.Close()
		return false
	}

	p.lConn = lc
	p.listenForMessages()
	// run message writers
	for i := uint8(0); i < p.nWriters; i++ {
		p.sendMessages(i)
	}

	p.keepAlive()
	p.healthMonitor()

	p.connected.Store(true)
	p.l.Info("Ready")

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
		p.l.Warn("Handshake: failed to generate nonce, send VER with 0 nonce", slog.String(errKey, err.Error()))
	}

	const lastBlock = int32(0)
	verMsg := wire.NewMsgVersion(me, you, nonce, lastBlock)

	if p.userAgentName != nil && p.userAgentVersion != nil {
		err = verMsg.AddUserAgent(*p.userAgentName, *p.userAgentVersion)
		if err != nil {
			p.l.Warn("Handshake: failed to add user agent, send VER without user agent", slog.String(errKey, err.Error()))
		}
	}

	err = wire.WriteMessage(c, verMsg, wire.ProtocolVersion, p.network)
	if err != nil {
		p.l.Error("Handshake failed.",
			slog.String("reason", "failed to write VER message"),
			slog.String(errKey, err.Error()),
		)

		return false
	}

	p.l.Debug("Sent", slogUpperString(commandKey, verMsg.Command()))

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

	receivedVerAck := false
	sentVerAck := false

handshakeLoop:
	for {
		// "read" next message
		readController <- struct{}{}

		select {
		// peer was stopped
		case <-p.execCtx.Done():
			return false

		case <-time.After(1 * time.Minute):
			p.l.Error("Handshake failed.", slog.String("reason", "handshake timeout"))
			return false

		case result := <-read:
			if result.err != nil {
				p.l.Error("Handshake failed.", slog.String(errKey, err.Error()))
				return false
			}

			nmsg := result.msg

			switch nmsg.Command() {
			case wire.CmdVerAck:
				p.l.Debug("Handshake: received VERACK")
				receivedVerAck = true

				if sentVerAck {
					break handshakeLoop
				}

			case wire.CmdVersion:
				p.l.Debug("Handshake: received VER")
				if sentVerAck {
					p.l.Warn("Handshake: received version message after sending verack.")
					continue
				}

				// send VERACK to node
				ackMsg := wire.NewMsgVerAck()
				err = wire.WriteMessage(c, ackMsg, wire.ProtocolVersion, p.network)
				if err != nil {
					p.l.Error("Handshake failed.",
						slog.String("reason", "failed to write VERACK message"),
						slog.String(errKey, err.Error()),
					)
					return false
				}

				p.l.Debug("Handshake: sent VERACK")
				sentVerAck = true

				if receivedVerAck {
					break handshakeLoop
				}

			default:
				p.l.Warn("Handshake: received unexpected message. Message was ignored", slogUpperString(commandKey, nmsg.Command()))
			}
		}
	}

	// if we exit the handshake loop, the handshake has completed successfully
	return true
}

func (p *Peer) keepAlive() {
	p.execWg.Add(1)

	go func() {
		defer p.execWg.Done()

		p.l.Debug("Start keep-alive")
		defer p.l.Debug("Stop keep-alive")

		t := time.NewTicker(p.pingInterval)
		defer t.Stop()

		for {
			select {
			case <-p.execCtx.Done():
				return
			case <-t.C:
				nonce, err := wire.RandomUint64()
				if err != nil {
					p.l.Error("KeepAlive: failed to generate nonce for PING message", slog.String(errKey, err.Error()))
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
		defer p.execWg.Done()

		p.l.Debug("Start health monitor")
		defer p.l.Debug("Stop health monitor")
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

			case <-t.C:
				// no ping or pong for too long
				p.l.Warn("Peer unhealthy - disconnecting")
				p.unhealthyDisconnect()
				return
			}
		}
	}()
}

func (p *Peer) disconnect() {
	p.l.Info("Disconnecting")

	p.cancelExecCtx()
	p.execWg.Wait()
	_ = p.lConn.Close()

	p.lConn = nil
	p.execCtx = nil
	p.cancelExecCtx = nil

	p.connected.Store(false)
	p.l.Info("Disconnected")
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
		l := p.l
		l.Debug("Starting read handler")
		defer l.Debug("Shutting down read handler")
		defer p.execWg.Done()

		reader := bufio.NewReader(&io.LimitedReader{R: p.lConn, N: p.maxMsgSize})
		for {
			msg, err := readWireMsg(p.execCtx, reader, wire.ProtocolVersion, p.network)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}

				l.Warn("Read issue - ignore msg", slog.String("err", err.Error()))
				continue
			}

			cmd := msg.Command()
			switch cmd {
			// ignore handshake type messages
			case wire.CmdVersion:
				fallthrough
			case wire.CmdVerAck:
				l.Warn("Received handshake message after handshake completed", slogUpperString(commandKey, cmd))

			// handle keep-alive ping-pong
			case wire.CmdPing:
				ping, ok := msg.(*wire.MsgPing)
				if !ok {
					p.l.Warn("Received invalid PING")
					continue
				}

				l.Log(context.Background(), slogLvlTrace, "Received", slogUpperString(commandKey, cmd))
				p.aliveCh <- struct{}{}
				p.writeCh <- wire.NewMsgPong(ping.Nonce) // are we sure it should go with write channel not beside?

			case wire.CmdPong:
				l.Log(context.Background(), slogLvlTrace, "Received", slogUpperString(commandKey, cmd))
				p.aliveCh <- struct{}{}

			// pass message to client
			default:
				l.Log(context.Background(), slogLvlTrace, "Received", slogUpperString(commandKey, cmd))
				p.mh.OnReceive(msg, p)
			}
		}
	}()
}

func (p *Peer) sendMessages(n uint8) {
	p.execWg.Add(1)

	go func() {
		l := p.l.With(slog.Int("instance", int(n)))

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

type readResult struct {
	msg wire.Message
	err error
}

func readWireMsg(ctx context.Context, r io.Reader, pver uint32, bsvnet wire.BitcoinNet) (wire.Message, error) {
	readMessageFinished := make(chan readResult, 1)

	go func() {
		msg, _, err := wire.ReadMessage(r, pver, bsvnet)
		readMessageFinished <- readResult{msg, err}
	}()

	// block until read complete or context is canceled
	select {
	case <-ctx.Done():
		return nil, ctx.Err()

	case readMsg := <-readMessageFinished:
		return readMsg.msg, readMsg.err
	}
}

func slogUpperString(key, val string) slog.Attr {
	return slog.String(key, strings.ToUpper(val))
}

const slogLvlTrace slog.Level = slog.LevelDebug - 4
