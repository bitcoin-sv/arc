package sql

import (
	"context"
	"testing"

	"github.com/bitcoin-sv/arc/blocktx/store"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type GetTransactionSourceSuite struct {
	DatabaseTestSuite
}

func (s GetTransactionSourceSuite) Test() {
	block := GetTestBlock()
	tx := GetTestTransaction()
	s.InsertBlock(block)

	s.InsertTransaction(tx)

	s.InsertBlockTransactionMap(&store.BlockTransactionMap{
		BlockID:       block.ID,
		TransactionID: tx.ID,
		Pos:           2,
	})

	store, err := NewPostgresStore(defaultParams)
	require.NoError(s.T(), err)

	h, err := chainhash.NewHash([]byte(tx.Hash))
	require.NoError(s.T(), err)

	source, err := store.GetTransactionSource(context.Background(), h)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), tx.Source, source)
}

func TestGetGetTransactionSourceSuite(t *testing.T) {
	s := new(GetTransactionSourceSuite)
	suite.Run(t, s)
	if err := recover(); err != nil {
		require.NoError(t, s.Database.Stop())
	}
}
