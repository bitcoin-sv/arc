package metamorph_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	storeMocks "github.com/bitcoin-sv/arc/internal/metamorph/store/mocks"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPeerHandler(t *testing.T) {
	messageCh := make(chan *metamorph.PeerTxMessage)
	mtmStore := &storeMocks.MetamorphStoreMock{
		GetRawTxsFunc: func(ctx context.Context, hashes [][]byte) ([][]byte, error) {
			rawTx := []byte("1234")
			return [][]byte{rawTx, rawTx}, nil
		},
	}

	peerHandler := metamorph.NewPeerHandler(mtmStore, messageCh)
	require.NotNil(t, peerHandler)

	peer, err := p2p.NewPeerMock("mock_peer", nil, wire.MainNet)
	require.NoError(t, err)

	t.Run("HandleTransactionSent", func(t *testing.T) {
		msgTx := wire.NewMsgTx(70001)
		hash := msgTx.TxHash()

		expectedMsg := &metamorph.PeerTxMessage{
			Hash:   &hash,
			Status: metamorph_api.Status_SENT_TO_NETWORK,
			Peer:   "mock_peer",
		}

		go func() {
			_ = peerHandler.HandleTransactionSent(msgTx, peer)
		}()

		select {
		case msg := <-messageCh:
			assert.Equal(t, expectedMsg, msg)
		case <-time.After(time.Second):
			t.Fatal("test timed out or error while executing goroutine")
		}
	})

	t.Run("HandleTransactionAnnouncement", func(t *testing.T) {
		hash, err := chainhash.NewHashFromStr("1234")
		require.NoError(t, err)

		msgInv := wire.NewInvVect(wire.InvTypeBlock, hash)
		require.NoError(t, err)

		expectedMsg := &metamorph.PeerTxMessage{
			Hash:   &msgInv.Hash,
			Status: metamorph_api.Status_SEEN_ON_NETWORK,
			Peer:   "mock_peer",
		}

		go func() {
			_ = peerHandler.HandleTransactionAnnouncement(msgInv, peer)
		}()

		select {
		case msg := <-messageCh:
			assert.Equal(t, expectedMsg, msg)
		case <-time.After(time.Second):
			t.Fatal("test timed out or error while executing goroutine")
		}
	})

	t.Run("HandleTransactionRejection", func(t *testing.T) {
		msgReject := wire.NewMsgReject("command", wire.RejectMalformed, "malformed")

		expectedMsg := &metamorph.PeerTxMessage{
			Hash:   &msgReject.Hash,
			Status: metamorph_api.Status_REJECTED,
			Peer:   "mock_peer",
			Err:    errors.New("transaction rejected by peer mock_peer: malformed"),
		}

		go func() {
			_ = peerHandler.HandleTransactionRejection(msgReject, peer)
		}()

		select {
		case msg := <-messageCh:
			assert.Equal(t, expectedMsg, msg)
		case <-time.After(time.Second):
			t.Fatal("test timed out or error while executing goroutine")
		}
	})

	t.Run("HandleTransactionsGet", func(t *testing.T) {
		txsCount := 2
		invMsgs := make([]*wire.InvVect, txsCount)
		expectedMsgs := make([]*metamorph.PeerTxMessage, txsCount)

		for i := 0; i < txsCount; i++ {
			hash, err := chainhash.NewHashFromStr("1234")
			require.NoError(t, err)

			msgInv := wire.NewInvVect(wire.InvTypeTx, hash)
			require.NoError(t, err)

			invMsgs[i] = msgInv

			expectedMsgs[i] = &metamorph.PeerTxMessage{
				Hash:   hash,
				Status: metamorph_api.Status_REQUESTED_BY_NETWORK,
				Peer:   "mock_peer",
			}
		}

		go func() {
			_, _ = peerHandler.HandleTransactionsGet(invMsgs, peer)
		}()

		counter := 0
		for i := 0; i < txsCount; i++ {
			select {
			case msg := <-messageCh:
				assert.Equal(t, expectedMsgs[counter], msg)
				counter++
			case <-time.After(5 * time.Second):
				t.Fatal("test timed out or error while executing goroutine")
			}
		}
	})

	t.Run("HandleTransaction", func(t *testing.T) {
		msgTx := wire.NewMsgTx(70001)
		hash := msgTx.TxHash()

		expectedMsg := &metamorph.PeerTxMessage{
			Hash:   &hash,
			Status: metamorph_api.Status_SEEN_ON_NETWORK,
			Peer:   "mock_peer",
		}

		go func() {
			_ = peerHandler.HandleTransaction(msgTx, peer)
		}()

		select {
		case msg := <-messageCh:
			assert.Equal(t, expectedMsg, msg)
		case <-time.After(time.Second):
			t.Fatal("test timed out or error while executing goroutine")
		}
	})
}
