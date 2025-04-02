package p2p_test

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cbeuw/connutil"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/p2p"
	"github.com/bitcoin-sv/arc/internal/p2p/mocks"
)

const (
	peerAddr   string          = "localhost:1234"
	bitcoinNet wire.BitcoinNet = wire.TestNet
)

var (
	blockHash, _ = chainhash.NewHashFromStr("00000000000007b1f872a8abe664223d65acd22a500b1b8eb5db3fe09a9837ff")
)

func Test_Connect(t *testing.T) {
	t.Run("Connect", func(t *testing.T) {
		// given

		toPeerConn, fromPeerConn := connutil.AsyncPipe()
		mhMq := &mocks.MessageHandlerIMock{OnSendFunc: func(_ wire.Message, _ p2p.PeerI) {}}

		sut := p2p.NewPeer(
			slog.Default(),
			mhMq,
			peerAddr,
			bitcoinNet,
			p2p.WithDialer(func(_, _ string) (net.Conn, error) {
				return toPeerConn, nil
			}),
		)

		// when
		var handshakeSuccess atomic.Bool
		go func() {
			res := testHandshake(t, fromPeerConn)
			handshakeSuccess.Store(res)
		}()

		result := sut.Connect()
		connected := sut.Connected()

		// give the "node" time to finish the handshake on its side
		time.Sleep(200 * time.Millisecond)

		// then
		require.True(t, handshakeSuccess.Load(), "Peer connection handshake failed")
		require.True(t, result, "Peer connection failed")
		require.True(t, connected, "Peer.Connected() returned `false` after successful connection to peer")
	})

	t.Run("Connect already connected peer", func(t *testing.T) {
		// given
		mhMq := &mocks.MessageHandlerIMock{OnSendFunc: func(_ wire.Message, _ p2p.PeerI) {}}
		sut, _, fromPeerConn := connectedPeer(t, mhMq)

		// when
		// call Connect() on connected peer and verify if it is connected and if handshake was perfomed second time

		var handshakeSuccess atomic.Bool
		go func() {
			res := testHandshake(t, fromPeerConn)
			handshakeSuccess.Store(res)
		}()

		result := sut.Connect()
		connected := sut.Connected()

		// give the "node" time to finish the handshake on its side
		time.Sleep(200 * time.Millisecond)

		// then
		require.False(t, handshakeSuccess.Load(), "Connected Peer shouldn't perform handshake again")
		require.True(t, result, "Peer connection failed")
		require.True(t, connected, "Peer.Connected() returned `false` after successful connection to peer")
	})
}

// TODO - don't know how to implement it yet
// func Test_Restart(t *testing.T) {
// }

func Test_Shutdown(t *testing.T) {
	t.Run("Shutdown", func(t *testing.T) {
		// given
		mhMq := &mocks.MessageHandlerIMock{OnSendFunc: func(_ wire.Message, _ p2p.PeerI) {}}
		sut, _, _ := connectedPeer(t, mhMq)

		// when
		sut.Shutdown()

		// then
		connected := sut.Connected()
		require.False(t, connected)
	})

	t.Run("Shutdown - connection should be closed", func(t *testing.T) {
		// given
		mhMq := &mocks.MessageHandlerIMock{OnSendFunc: func(_ wire.Message, _ p2p.PeerI) {}}
		sut, toPeerConn, fromPeerConn := connectedPeer(t, mhMq)

		// when
		sut.Shutdown()
		connected := sut.Connected()
		require.False(t, connected)

		// try to sent data through connection
		_, writeToNodeErr := toPeerConn.Write([]byte{})

		// try to send message as a node
		invMsg := wire.NewMsgInv()
		nodeWriteToMeErr := wire.WriteMessage(fromPeerConn, invMsg, wire.ProtocolVersion, bitcoinNet)

		// then
		require.ErrorIs(t, io.ErrClosedPipe, writeToNodeErr)
		require.ErrorIs(t, io.ErrClosedPipe, nodeWriteToMeErr)
	})
}

func Test_String(t *testing.T) {
	t.Run("String - returns address", func(t *testing.T) {
		// given
		sut := p2p.NewPeer(slog.Default(), nil, peerAddr, bitcoinNet)

		// when
		result := sut.String()

		// then
		require.Equal(t, peerAddr, result)
	})
}

func Test_keepAlive(t *testing.T) {
	t.Run("ping-pong", func(t *testing.T) {
		// given
		const (
			keepAliveThreshold = 200 * time.Millisecond
			pingPongInterval   = 20 * time.Millisecond
		)

		mhMq := &mocks.MessageHandlerIMock{OnSendFunc: func(_ wire.Message, _ p2p.PeerI) {}}
		sut, _, fromPeerConn := peerWithConn(t, mhMq, p2p.WithPingInterval(pingPongInterval, keepAliveThreshold))

		var sutUnhealthy atomic.Bool
		go func() {
			<-sut.IsUnhealthyCh()
			sutUnhealthy.Store(true)
		}()

		connectPeer(t, sut, fromPeerConn)
		go pong(t, fromPeerConn)

		// when
		// give peers time to play ping-pong
		time.Sleep(time.Second)

		// then
		// peer should not disconnect or signal it's unhealthy
		require.True(t, sut.Connected(), "Peer shouldn't disconnect")
		require.False(t, sutUnhealthy.Load(), "Peer shouldn't signal it's unhealthy")
	})

	t.Run("no ping-pong - should disconnect", func(t *testing.T) {
		// given
		const (
			keepAliveThreshold = 10 * time.Millisecond
			pingPongInterval   = time.Second
		)

		mhMq := &mocks.MessageHandlerIMock{OnSendFunc: func(_ wire.Message, _ p2p.PeerI) {}}
		sut, _, fromPeerConn := peerWithConn(t, mhMq, p2p.WithPingInterval(pingPongInterval, keepAliveThreshold))

		var sutUnhealthy atomic.Bool
		go func() {
			<-sut.IsUnhealthyCh()
			sutUnhealthy.Store(true)
		}()

		connectPeer(t, sut, fromPeerConn)

		// when
		// give peers time to play ping-pong
		time.Sleep(100 * time.Millisecond)

		// then
		// peer should disconnect on invalid message and signal it's unhealthy
		require.False(t, sut.Connected(), "Peer didn't disconnect when no ping-pong")
		require.True(t, sutUnhealthy.Load(), "Peer didn't signal it's unhealthy when no ping-pong")
	})
}

func Test_WriteMsg(t *testing.T) {
	tt := []struct {
		cmdType string
		msg     wire.Message
	}{
		{
			cmdType: wire.CmdGetBlocks,
			msg:     wire.NewMsgGetBlocks(blockHash),
		},
		// TODO: add rest msg types
	}

	for _, tc := range tt {
		t.Run(fmt.Sprintf("Write %s", tc.cmdType), func(t *testing.T) {
			// given
			mhMq := &mocks.MessageHandlerIMock{
				OnSendFunc: func(_ wire.Message, _ p2p.PeerI) {},
			}

			sut, _, fromPeerConn := connectedPeer(t, mhMq)

			// when
			sut.WriteMsg(tc.msg)

			// read msg as node
			readMsg, _, readErr := wire.ReadMessage(fromPeerConn, wire.ProtocolVersion, bitcoinNet)

			// then
			require.NoError(t, readErr)
			require.Equal(t, tc.cmdType, readMsg.Command())

			require.Equal(t, 1, len(mhMq.OnSendCalls()))
		})
	}
}

func Test_listenMessages(t *testing.T) {
	tt := []struct {
		cmdType string
		msg     wire.Message
	}{
		{
			cmdType: wire.CmdGetBlocks,
			msg:     wire.NewMsgGetBlocks(blockHash),
		},
		// TODO: add other messages
	}

	for _, tc := range tt {
		t.Run(fmt.Sprintf("Receive %s", tc.cmdType), func(t *testing.T) {
			// given
			var receiveMsgWg sync.WaitGroup

			mhMq := &mocks.MessageHandlerIMock{
				OnReceiveFunc: func(msg wire.Message, _ p2p.PeerI) {
					// check if received expected msg
					require.Equal(t, tc.cmdType, msg.Command())
					receiveMsgWg.Done()
				},
				OnSendFunc: func(_ wire.Message, _ p2p.PeerI) {},
			}

			_, _, fromPeerConn := connectedPeer(t, mhMq)

			// when
			// send msg from "node"
			writeErr := wire.WriteMessage(fromPeerConn, tc.msg, wire.ProtocolVersion, bitcoinNet)
			receiveMsgWg.Add(1)

			// then
			require.NoError(t, writeErr)

			receiveMsgWg.Wait()
			require.Equal(t, 1, len(mhMq.OnReceiveCalls()))
		})
	}
}

func Test_ErrorOnRead(t *testing.T) {
	t.Run("Error while reading message from node - should disconnect", func(t *testing.T) {
		// given
		mhMq := &mocks.MessageHandlerIMock{OnSendFunc: func(_ wire.Message, _ p2p.PeerI) {}}
		sut, _, fromPeerConn := connectedPeer(t, mhMq)

		var sutUnhealthy atomic.Bool
		go func() {
			<-sut.IsUnhealthyCh()
			sutUnhealthy.Store(true)
		}()

		invalidPayload := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25}

		// when
		// send ivnalid msg from "node"
		n, writeErr := fromPeerConn.Write(invalidPayload)

		// then
		// give peer time to consume msg from node
		time.Sleep(100 * time.Millisecond)

		require.NoError(t, writeErr)
		require.Equal(t, len(invalidPayload), n)

		// peer should disconnect on invalid message and signal it's unhealthy
		require.False(t, sut.Connected(), "Peer didn't disconnect on error on reading message")
		require.True(t, sutUnhealthy.Load(), "Peer didn't signal it's unhealthy on error on reading message")
	})
}

func Test_ErrorOnWrite(t *testing.T) {
	t.Run("Error while writing message to node - should disconnect", func(t *testing.T) {
		// given
		mhMq := &mocks.MessageHandlerIMock{OnSendFunc: func(_ wire.Message, _ p2p.PeerI) {}}
		sut, _, _ := connectedPeer(t, mhMq)

		var sutUnhealthy atomic.Bool
		go func() {
			<-sut.IsUnhealthyCh()
			sutUnhealthy.Store(true)
		}()

		invalidMsgMq := &mocks.MessageMock{
			CommandFunc: func() string { return "sample" },
			BsvEncodeFunc: func(_ io.Writer, _ uint32, _ wire.MessageEncoding) error {
				return errors.New("invalid wire message sample")
			},
		}

		// when
		sut.WriteMsg(invalidMsgMq)

		// then
		// give peer time to consume msg from node
		time.Sleep(100 * time.Millisecond)

		// peer should disconnect on invalid message and signal it's unhealthy
		require.False(t, sut.Connected(), "Peer didn't disconnect on error on writing message")
		require.True(t, sutUnhealthy.Load(), "Peer didn't signal it’s unhealthy on error while writing message")
	})
}

func connectedPeer(t *testing.T, msgHandler p2p.MessageHandlerI, opts ...p2p.PeerOptions) (peer *p2p.Peer, toPeerConn, fromPeerConn net.Conn) {
	t.Helper()

	peer, toPeerConn, fromPeerConn = peerWithConn(t, msgHandler, opts...)
	connectPeer(t, peer, fromPeerConn)

	return
}

func peerWithConn(t *testing.T, msgHandler p2p.MessageHandlerI, opts ...p2p.PeerOptions) (peer *p2p.Peer, toPeerConn, fromPeerConn net.Conn) {
	t.Helper()

	toPeerConn, fromPeerConn = connutil.AsyncPipe()

	peer = p2p.NewPeer(
		slog.Default(),
		msgHandler,
		peerAddr,
		bitcoinNet,
		append(opts,
			p2p.WithDialer(func(_, _ string) (net.Conn, error) {
				return toPeerConn, nil
			}),
		)...,
	)

	return peer, toPeerConn, fromPeerConn
}

func connectPeer(t *testing.T, peer *p2p.Peer, fromPeerConn net.Conn) {
	t.Helper()

	// when
	var handshakeSuccess atomic.Bool
	go func() {
		res := testHandshake(t, fromPeerConn)
		handshakeSuccess.Store(res)
	}()

	result := peer.Connect()
	connected := peer.Connected()

	// give the "node" time to finish the handshake on its side
	time.Sleep(100 * time.Millisecond)

	// then
	require.True(t, handshakeSuccess.Load(), "Peer connection handshake failed")
	require.True(t, result, "Peer connection failed")
	require.True(t, connected, "Peer.Connected() returned `false` after successful connection to peer")
}

func testHandshake(t *testing.T, conn net.Conn) (ok bool) {
	t.Helper()

	/* 1. wait for VER
	 * 2. send VERACK
	 * 3. send VER
	 * 4. wait for VERACK
	 */

	// read VER
	msg, _, err := wire.ReadMessage(conn, wire.ProtocolVersion, bitcoinNet)
	require.NoError(t, err, "Error during reading VER message from host/client??")
	require.Equal(t, wire.CmdVersion, msg.Command(), "Received other msg from host/client than VER in handshake")

	verMsg, ok := msg.(*wire.MsgVersion)
	require.True(t, ok)
	require.Equal(t, int32(wire.ProtocolVersion), verMsg.ProtocolVersion)

	// send VERACK
	ackMsg := wire.NewMsgVerAck()
	err = wire.WriteMessage(conn, ackMsg, wire.ProtocolVersion, bitcoinNet)
	require.NoError(t, err)

	// send VER
	me := wire.NewNetAddress(&net.TCPAddr{IP: nil, Port: 0}, wire.SFNodeNetwork)

	nAddr, _ := net.ResolveTCPAddr("tcp", "localhost:8876")
	you := wire.NewNetAddress(nAddr, wire.SFNodeNetwork)

	nonce, err := wire.RandomUint64()
	require.NoError(t, err)

	verMsg = wire.NewMsgVersion(me, you, nonce, 0)
	err = wire.WriteMessage(conn, verMsg, wire.ProtocolVersion, bitcoinNet)
	require.NoError(t, err, "Error during sending VER from peer")

	// read VERACK
	msg, _, err = wire.ReadMessage(conn, wire.ProtocolVersion, bitcoinNet)
	require.NoError(t, err, "Error during reading VERACK message from host/client??")
	require.Equal(t, wire.CmdVerAck, msg.Command(), "Received other msg from host/client than VERACK in handshake")

	return true
}

func pong(t *testing.T, conn net.Conn) {
	t.Helper()

	for {
		msg, _, err := wire.ReadMessage(conn, wire.ProtocolVersion, bitcoinNet)
		if err != nil {
			require.NoError(t, err)
		}

		if msg.Command() == wire.CmdPing {
			ping, ok := msg.(*wire.MsgPing)
			require.True(t, ok)

			pong := wire.NewMsgPong(ping.Nonce)

			pongErr := wire.WriteMessage(conn, pong, wire.ProtocolVersion, bitcoinNet)
			require.NoError(t, pongErr)
		}
	}
}
