package mcast

import (
	"errors"
	"log/slog"

	"github.com/bitcoin-sv/arc/internal/metamorph/bcnet/metamorph_p2p"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/multicast"
	"github.com/libsv/go-p2p/bsvutil"
	"github.com/libsv/go-p2p/wire"
)

var ErrTxRejectedByPeer = errors.New("transaction rejected by peer")

var _ multicast.MessageHandlerI = (*Multicaster)(nil)

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
		//	m.logger.Error("failed to parse tx", slog.String("rawHex", hex.EncodeToString(rawTx)), slog.String("err", err.Error()))
		return err
	}

	m.txGroup.WriteMsg(tx.MsgTx())
	return nil
}

// OnReceive should be fire & forget
func (m *Multicaster) OnReceive(msg wire.Message) {
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

// OnSend should be fire & forget
func (m *Multicaster) OnSend(msg wire.Message) {
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
