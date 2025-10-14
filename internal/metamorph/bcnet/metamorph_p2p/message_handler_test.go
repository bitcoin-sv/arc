package metamorph_p2p

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/libsv/go-p2p/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	storeMocks "github.com/bitcoin-sv/arc/internal/metamorph/store/mocks"
	p2pMocks "github.com/bitcoin-sv/arc/internal/p2p/mocks"
	"github.com/bitcoin-sv/arc/internal/testdata"
	chhash "github.com/bsv-blockchain/go-bt/v2/chainhash"
)

const (
	bitconnet = wire.TestNet
	peerAddr  = "peer"
)

var (
	txHash    = testdata.TX1Hash
	txHashB   = testdata.TX1HashB
	blockHash = testdata.Block1Hash
)

func Test_MessageHandlerOnReceive(t *testing.T) {
	ch, err := chhash.NewHashFromStr(ptrTo(wire.NewMsgTx(70001).TxHash()).String())
	require.NoError(t, err)
	ch2, err := chhash.NewHashFromStr(ptrTo(wire.NewMsgReject("command", wire.RejectMalformed, "malformed").Hash).String())
	require.NoError(t, err)

	tt := []struct {
		name                 string
		wireMsg              wire.Message
		expectedOnChannelMsg *TxStatusMessage
		ignore               bool
	}{
		{
			name:    wire.CmdTx,
			wireMsg: wire.NewMsgTx(70001),
			expectedOnChannelMsg: &TxStatusMessage{
				Hash:          ch,
				Status:        metamorph_api.Status_SEEN_ON_NETWORK,
				Peer:          peerAddr,
				ReceivedRawTx: true,
			},
		},
		{
			name: wire.CmdInv,
			wireMsg: func() wire.Message {
				msg := wire.NewMsgInv()
				_ = msg.AddInvVect(wire.NewInvVect(wire.InvTypeTx, txHash))
				return msg
			}(),

			expectedOnChannelMsg: &TxStatusMessage{
				Hash:   testdata.TX1HashB,
				Status: metamorph_api.Status_SEEN_ON_NETWORK,
				Peer:   peerAddr,
			},
		},
		{
			name: wire.CmdInv + " BLOCK should ignore",
			wireMsg: func() wire.Message {
				msg := wire.NewMsgInv()
				_ = msg.AddInvVect(wire.NewInvVect(wire.InvTypeBlock, blockHash))
				return msg
			}(),
			ignore: true,
		},
		{
			name:    wire.CmdReject,
			wireMsg: wire.NewMsgReject("command", wire.RejectMalformed, "malformed"),
			expectedOnChannelMsg: &TxStatusMessage{
				Hash:   ch2,
				Status: metamorph_api.Status_REJECTED,
				Peer:   peerAddr,
				Err:    errors.Join(ErrTxRejectedByPeer, fmt.Errorf("peer: %s reason: %s", peerAddr, "malformed")),
			},
		},
		{
			name: wire.CmdGetData,
			wireMsg: func() wire.Message {
				msg := wire.NewMsgGetData()
				// add block inv
				_ = msg.AddInvVect(wire.NewInvVect(wire.InvTypeBlock, blockHash))
				// add tx inv
				_ = msg.AddInvVect(wire.NewInvVect(wire.InvTypeTx, txHash))

				return msg
			}(),
			expectedOnChannelMsg: &TxStatusMessage{
				Hash:   txHashB,
				Status: metamorph_api.Status_REQUESTED_BY_NETWORK,
				Peer:   peerAddr,
			},
		},
	}

	for _, tc := range tt {
		t.Run(fmt.Sprintf("Received %s", tc.name), func(t *testing.T) {
			// given
			messageCh := make(chan *TxStatusMessage, 10)
			store := &storeMocks.MetamorphStoreMock{
				GetRawTxsFunc: func(_ context.Context, _ [][]byte) ([][]byte, error) {
					return [][]byte{
						testdata.TX1Raw.Bytes(),
					}, nil
				},
			}
			peer := &p2pMocks.PeerIMock{
				StringFunc:   func() string { return peerAddr },
				WriteMsgFunc: func(_ wire.Message) {},
			}

			sut := NewMsgHandler(slog.Default(), store, messageCh, WithNow(func() time.Time {
				return time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
			}))

			// when
			sut.OnReceive(tc.wireMsg, peer)

			// then
			select {
			case msg := <-messageCh:
				if tc.ignore {
					t.Fatal("MsgHandler react on message it should ignore")
				}

				assert.Equal(t, tc.expectedOnChannelMsg, msg)

			case <-time.After(time.Second):
				if !tc.ignore {
					t.Fatal("test timed out or error while executing goroutine")
				}
			}
		})
	}
}

func Test_MessageHandlerOnSend(t *testing.T) {
	ch3, err := chhash.NewHashFromStr(ptrTo(wire.NewMsgTx(70001).TxHash()).String())
	require.NoError(t, err)
	tt := []struct {
		name                 string
		wireMsg              wire.Message
		expectedOnChannelMsg *TxStatusMessage
		ignore               bool
	}{
		{
			name:    wire.CmdTx,
			wireMsg: wire.NewMsgTx(70001),
			expectedOnChannelMsg: &TxStatusMessage{
				Hash:   ch3,
				Status: metamorph_api.Status_SENT_TO_NETWORK,
				Peer:   peerAddr,
			},
		},
		{
			name: wire.CmdInv + " should ignore",
			wireMsg: func() wire.Message {
				msg := wire.NewMsgInv()
				_ = msg.AddInvVect(wire.NewInvVect(wire.InvTypeTx, txHash))
				return msg
			}(),
			ignore: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			messageCh := make(chan *TxStatusMessage, 10)
			store := &storeMocks.MetamorphStoreMock{}
			peer := &p2pMocks.PeerIMock{
				StringFunc: func() string { return peerAddr },
			}

			sut := NewMsgHandler(slog.Default(), store, messageCh, WithNow(func() time.Time {
				return time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
			}))

			// when
			sut.OnSend(tc.wireMsg, peer)

			// then
			select {
			case msg := <-messageCh:
				if tc.ignore {
					t.Fatal("MsgHandler react on message it should ignore")
				}

				assert.Equal(t, tc.expectedOnChannelMsg, msg)

			case <-time.After(time.Second):
				if !tc.ignore {
					t.Fatal("test timed out or error while executing goroutine")
				}
			}
		})
	}
}

func ptrTo[T any](v T) *T {
	return &v
}
