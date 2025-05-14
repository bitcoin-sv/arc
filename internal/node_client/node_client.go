package node_client

import (
	"context"
	"errors"
	"math/big"
	"runtime"
	"strings"
	"time"

	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/ccoveille/go-safecast"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bitcoin-sv/arc/pkg/rpc_client"
	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/pkg/tracing"
)

var (
	ErrFailedToGetRawTransaction   = errors.New("failed to get raw transaction")
	ErrFailedToGetMempoolAncestors = errors.New("failed to get mempool ancestors")
	ErrFailedToGetBlockTxIDs       = errors.New("failed to get block tx IDs")
)

type NodeClient struct {
	bitcoinClient     *rpc_client.RPCClient
	tracingEnabled    bool
	tracingAttributes []attribute.KeyValue
}

func WithTracer(attr ...attribute.KeyValue) func(s *NodeClient) {
	return func(p *NodeClient) {
		p.tracingEnabled = true
		if len(attr) > 0 {
			p.tracingAttributes = append(p.tracingAttributes, attr...)
		}
		_, file, _, ok := runtime.Caller(1)
		if ok {
			p.tracingAttributes = append(p.tracingAttributes, attribute.String("file", file))
		}
	}
}

func New(n *rpc_client.RPCClient, opts ...func(client *NodeClient)) (NodeClient, error) {
	node := NodeClient{
		bitcoinClient: n,
	}

	for _, opt := range opts {
		opt(&node)
	}

	return node, nil
}

func (n NodeClient) GetMempoolAncestors(ctx context.Context, ids []string) (allTxIDs []string, err error) {
	_, span := tracing.StartTracing(ctx, "NodeClient_GetMempoolAncestors", n.tracingEnabled, n.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	uniqueIDs := make(map[string]struct{})

	for _, id := range ids {
		_, getMemPoolAncSpan := tracing.StartTracing(ctx, "Bitcoind_GetMempoolAncestors", n.tracingEnabled, n.tracingAttributes...)
		txIDs, err := n.bitcoinClient.GetMempoolAncestors(ctx, id)
		tracing.EndTracing(getMemPoolAncSpan, nil)
		if err != nil {
			if strings.Contains(err.Error(), "Transaction not in mempool") {
				continue
			}

			return nil, errors.Join(ErrFailedToGetMempoolAncestors, err)
		}

		for _, txID := range txIDs {
			uniqueIDs[txID] = struct{}{}
		}

		uniqueIDs[id] = struct{}{}
	}

	allTxIDs = make([]string, len(uniqueIDs))
	counter := 0
	for id := range uniqueIDs {
		allTxIDs[counter] = id
		counter++
	}
	return allTxIDs, nil
}

func (n NodeClient) GetRawTransaction(ctx context.Context, id string) (rt *sdkTx.Transaction, err error) {
	_, span := tracing.StartTracing(ctx, "NodeClient_GetRawTransaction", n.tracingEnabled, n.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	nTx, err := n.bitcoinClient.GetRawTransactionHex(ctx, id)
	if err != nil {
		return nil, errors.Join(ErrFailedToGetRawTransaction, err)
	}

	rt, err = sdkTx.NewTransactionFromHex(nTx)
	if err != nil {
		return nil, err
	}

	return rt, nil
}

func (n NodeClient) GetBlock(ctx context.Context, id string) (message *blocktx.BlockMessage, err error) {
	_, span := tracing.StartTracing(ctx, "NodeClient_GetBlockTxIDs", n.tracingEnabled, n.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	block, err := n.bitcoinClient.GetBlock(ctx, id)
	if err != nil {
		return nil, errors.Join(ErrFailedToGetBlockTxIDs, err)
	}

	return blockToBlockMessage(block)
}

func blockToBlockMessage(block *Block) (*blocktx.BlockMessage, error) {
	blockHash, err := chainhash.NewHashFromStr(block.Hash)
	if err != nil {
		return nil, err
	}
	prevBlockHash, err := chainhash.NewHashFromStr(block.PreviousBlockHash)
	if err != nil {
		return nil, err
	}
	merkleRoot, err := chainhash.NewHashFromStr(block.MerkleRoot)
	if err != nil {
		return nil, err
	}

	version, err := safecast.ToInt32(block.Version)
	if err != nil {
		return nil, err
	}

	unixTimestamp, err := safecast.ToInt64(block.Time)
	if err != nil {
		return nil, err
	}

	n := new(big.Int)
	n.SetString(block.Bits, 16)
	bits, err := safecast.ToUint32(n.Int64())
	if err != nil {
		return nil, err
	}

	header := &blocktx.BlockHeader{
		Version:    version,
		PrevBlock:  *prevBlockHash,
		MerkleRoot: *merkleRoot,
		Timestamp:  time.Unix(unixTimestamp, 0),
		Bits:       bits,
		Nonce:      block.Nonce,
	}

	txHashes := make([]*chainhash.Hash, len(block.Tx))

	for i, tx := range block.Tx {
		txHash, err := chainhash.NewHashFromStr(tx)
		if err != nil {
			return nil, err
		}

		txHashes[i] = txHash
	}

	b := &blocktx.BlockMessage{
		Hash:              blockHash,
		Header:            header,
		Height:            block.Height,
		TransactionHashes: txHashes,
		Size:              block.Size,
	}

	return b, nil
}
