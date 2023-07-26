package sql

import (
	"testing"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/stretchr/testify/require"
)

func TestGetFullQuery(t *testing.T) {
	t.Run("test", func(t *testing.T) {
		hash1, err := chainhash.NewHashFromStr("181fcd0be5a1742aabd594a5bfd5a1e7863a4583290da72fb2a896dfa824645c")
		require.NoError(t, err)
		hash2, err := chainhash.NewHashFromStr("2e5c318f7f2e2e80e484ca1f00f1b7bee95a33a848de572a304b973ff2b0b35b")
		require.NoError(t, err)
		hash3, err := chainhash.NewHashFromStr("82b0a66c5dcbd0f6f6f99e2bf766e1d40b04c175c01ee87f1abc36136e511a7e")
		require.NoError(t, err)

		expectedQuery := `
SELECT
b.hash, b.height, t.hash
FROM blocks b
INNER JOIN block_transactions_map m ON m.blockid = b.id
INNER JOIN transactions t ON m.txid = t.id
WHERE t.hash in ('\x5c6424a8df96a8b22fa70d2983453a86e7a1d5bfa594d5ab2a74a1e50bcd1f18','\x5bb3b0f23f974b302a57de48a8335ae9beb7f1001fca84e4802e2e7f8f315c2e','\x7e1a516e1336bc1a7fe81ec075c1040bd4e166f72b9ef9f6f6d0cb5d6ca6b082')
AND b.orphanedyn = false`

		transactions := &blocktx_api.Transactions{
			Transactions: []*blocktx_api.Transaction{
				{
					Hash: hash1.CloneBytes(),
				},
				{
					Hash: hash2.CloneBytes(),
				},
				{
					Hash: hash3.CloneBytes(),
				},
			},
		}

		q := getFullQuery(transactions)
		require.Equal(t, expectedQuery, q)
	})
}
