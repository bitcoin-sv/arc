package p2p

import (
	"bufio"
	"context"
	"io"
	"strings"

	"github.com/libsv/go-p2p/wire"
)

type WireReader struct {
	bufio.Reader
	limitedReader *io.LimitedReader
	maxMsgSize    int64
}

func NewWireReader(r io.Reader, maxMsgSize int64) *WireReader {
	lr := &io.LimitedReader{R: r, N: maxMsgSize}

	return &WireReader{
		Reader:        *bufio.NewReader(lr),
		limitedReader: lr,
		maxMsgSize:    maxMsgSize,
	}
}

func NewWireReaderSize(r io.Reader, maxMsgSize int64, buffSize int) *WireReader {
	lr := &io.LimitedReader{R: r, N: maxMsgSize}

	return &WireReader{
		Reader:        *bufio.NewReaderSize(lr, buffSize),
		limitedReader: lr,
		maxMsgSize:    maxMsgSize,
	}
}

func (r *WireReader) ReadNextMsg(ctx context.Context, pver uint32, network wire.BitcoinNet) (wire.Message, error) {
	result := make(chan readResult, 1)
	go handleRead(r, pver, network, result)

	// block until read complete or context is canceled
	select {
	case <-ctx.Done():
		return nil, ctx.Err()

	case readMsg := <-result:
		return readMsg.msg, readMsg.err
	}
}

func (r *WireReader) resetLimit() {
	r.limitedReader.N = r.maxMsgSize
}

type readResult struct {
	msg wire.Message
	err error
}

func handleRead(r *WireReader, pver uint32, bsvnet wire.BitcoinNet, result chan<- readResult) {
	for {
		msg, _, err := wire.ReadMessage(r, pver, bsvnet)
		r.resetLimit()

		if err != nil {
			if strings.Contains(err.Error(), "unhandled command [") { // TODO: change it with new go-p2p version
				// ignore unknown msg
				continue
			}
		}

		result <- readResult{msg, err}
		return
	}
}
