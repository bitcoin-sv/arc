package multicast_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/multicast"
	"github.com/bitcoin-sv/arc/internal/multicast/mocks"
	"github.com/libsv/go-p2p/wire"
	"github.com/stretchr/testify/require"
)

var (
	addr  = "[ff02::1]:1234"
	bcNet = wire.TestNet
)

func TestGroupCommunication(t *testing.T) {
	// given
	lMsgHandler := &mocks.MessageHandlerIMock{OnReceiveFromMcastFunc: func(_ wire.Message) {}}
	listener := multicast.NewGroup[*wire.MsgPing](slog.Default(), lMsgHandler, addr, multicast.Read, bcNet)
	require.True(t, listener.Connect())
	defer listener.Disconnect()

	wMsgHandler := &mocks.MessageHandlerIMock{OnSendToMcastFunc: func(_ wire.Message) {}}
	writer := multicast.NewGroup[*wire.MsgPing](slog.Default(), wMsgHandler, addr, multicast.Write, bcNet)
	require.True(t, writer.Connect())
	defer writer.Disconnect()

	msg := wire.NewMsgPing(825906425)

	// when
	writer.WriteMsg(msg)
	time.Sleep(200 * time.Millisecond)

	// then
	sentMsgs := wMsgHandler.OnSendToMcastCalls()
	require.Len(t, sentMsgs, 1, "writer didn't send message")
	require.Equal(t, msg, (sentMsgs[0].Msg).(*wire.MsgPing))

	receivedMsgs := lMsgHandler.OnReceiveFromMcastCalls()
	require.Len(t, receivedMsgs, 1, "listener didn't receive message")
	require.Equal(t, msg, (receivedMsgs[0].Msg).(*wire.MsgPing))
}
