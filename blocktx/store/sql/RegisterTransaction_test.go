package sql

import (
	"context"
	"testing"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegister(t *testing.T) {
	s, err := New("sqlite_memory")

	require.NoError(t, err)

	source, _, blockHeight, err := s.RegisterTransaction(context.Background(), &blocktx_api.TransactionAndSource{
		Hash:   []byte("test transaction hash 1"),
		Source: "TEST",
	})

	assert.NoError(t, err)
	assert.Equal(t, "TEST", source)
	assert.Zero(t, blockHeight)

	source, _, blockHeight, err = s.RegisterTransaction(context.Background(), &blocktx_api.TransactionAndSource{
		Hash:   []byte("test transaction hash 1"),
		Source: "TEST2",
	})

	assert.NoError(t, err)
	assert.Equal(t, "TEST", source)
	assert.Zero(t, blockHeight)

}
