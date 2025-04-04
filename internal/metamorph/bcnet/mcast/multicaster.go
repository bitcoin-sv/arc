package mcast

import (
	"errors"
	"log/slog"

	"github.com/libsv/go-p2p/bsvutil"
	"github.com/libsv/go-p2p/wire"

	"github.com/bitcoin-sv/arc/internal/metamorph/bcnet/metamorph_p2p"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/multicast"
)

var ErrTxRejectedByPeer = errors.New("transaction rejected by peer")

var _ multicast.MessageHandlerI = (*Multicaster)(nil)

// Multicaster facilitates the transmission and reception of transaction and reject messages over multicast groups.
//
// Fields:
// - `messageCh`: Channel used to send status messages for transactions, such as acceptance or rejection.
// - `txGroup`: Multicast group for transmitting transaction (`MsgTx`) messages.
// - `rejectGroup`: Multicast group for receiving rejection (`MsgReject`) messages.
//
// Responsibilities:
// - Establishes and manages connections to multicast groups for sending and receiving blockchain transaction messages.
// - Handles the transmission of transactions and processes rejections received from the network.
//
// Key Methods:
// - `NewMulticaster`: Initializes a new `Multicaster` instance with specified multicast group addresses, network, and message channel.
// - `Connect`: Connects to both `txGroup` (for sending) and `rejectGroup` (for receiving).
// - `Disconnect`: Disconnects from all multicast groups, cleaning up resources.
// - `SendTx`: Encodes and sends a raw transaction to the multicast `txGroup`.
// - `OnReceive`: Processes messages received via the `rejectGroup` and updates the `messageCh` with rejection status.
// - `OnSend`: Processes messages sent via the `txGroup` and updates the `messageCh` with sent-to-network status.
//
// Behavior:
// - On receiving a `MsgReject` (`CmdReject`):
//  1. Extracts the rejection reason and transaction hash.
//  2. Sends a `TxStatusMessage` to the `messageCh` indicating rejection status.
//
// - On sending a `MsgTx` (`CmdTx`):
//  1. Extracts the transaction hash.
//  2. Sends a `TxStatusMessage` to the `messageCh` indicating the transaction was sent to the network.
//
// - Ignores unsupported or irrelevant message types for both sending and receiving.
type Multicaster struct {
	logger    *slog.Logger
	messageCh chan<- *metamorph_p2p.TxStatusMessage

	txGroup     *multicast.Group[*wire.MsgTx]
	rejectGroup *multicast.Group[*wire.MsgReject]
}

type GroupsAddresses struct {
	McastTx     string
	McastReject string
}

func NewMulticaster(l *slog.Logger, addresses GroupsAddresses, network wire.BitcoinNet, messageCh chan<- *metamorph_p2p.TxStatusMessage) *Multicaster {
	m := Multicaster{
		logger:    l.With("module", "multicaster"),
		messageCh: messageCh,
	}

	m.txGroup = multicast.NewGroup[*wire.MsgTx](l, &m, addresses.McastTx, multicast.Write, network)
	m.rejectGroup = multicast.NewGroup[*wire.MsgReject](l, &m, addresses.McastReject, multicast.Read, network)

	return &m
}

func (m *Multicaster) Connect() bool {
	return m.txGroup.Connect() && m.rejectGroup.Connect()
}

func (m *Multicaster) Disconnect() {
	m.txGroup.Disconnect()
	m.rejectGroup.Disconnect()
}

func (m *Multicaster) SendTx(rawTx []byte) error {
	tx, err := bsvutil.NewTxFromBytes(rawTx)
	if err != nil {
		return err
	}

	m.txGroup.WriteMsg(tx.MsgTx())
	return nil
}

// OnReceiveFromMcast should be fire & forget
func (m *Multicaster) OnReceiveFromMcast(msg wire.Message) {
	if msg.Command() == wire.CmdReject {
		rejectMsg, ok := msg.(*wire.MsgReject)
		if !ok {
			return
		}

		m.messageCh <- &metamorph_p2p.TxStatusMessage{
			Hash:   &rejectMsg.Hash,
			Status: metamorph_api.Status_REJECTED,
			Peer:   "Mcast REJECT",
			Err:    errors.Join(ErrTxRejectedByPeer, errors.New(rejectMsg.Reason)),
		}
	}

	// ignore other messages
}

// OnSendToMcast should be fire & forget
func (m *Multicaster) OnSendToMcast(msg wire.Message) {
	if msg.Command() == wire.CmdTx {
		txMsg, ok := msg.(*wire.MsgTx)
		if !ok {
			return
		}

		hash := txMsg.TxHash()
		m.messageCh <- &metamorph_p2p.TxStatusMessage{
			Hash:   &hash,
			Status: metamorph_api.Status_SENT_TO_NETWORK,
			Peer:   "Mcast TX",
		}
	}

	// ignore other messages
}
