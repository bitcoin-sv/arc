package integrationtest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	testutils "github.com/bitcoin-sv/arc/internal/test_utils"
)

func TestMerklePaths(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	t.Run("request unregistered tx", func(t *testing.T) {
		// given
		defer pruneTables(t, dbConn)
		testutils.LoadFixtures(t, dbConn, "fixtures/merkle_paths")

		processor, _, _, registerTxChannel, publishedTxsCh := setupSut(t, dbInfo)

		txWithoutMerklePath := testutils.RevChainhash(t, "cd3d2f97dfc0cdb6a07ec4b72df5e1794c9553ff2f62d90ed4add047e8088853")
		expectedMerklePath := "fefe8a0c0003020002cd3d2f97dfc0cdb6a07ec4b72df5e1794c9553ff2f62d90ed4add047e8088853010021132d32cb5411c058bb4391f24f6a36ed9b810df851d0e36cac514fd03d6b4e010100f883cc2d3bb5d4485accaa3502cf834934420616d8556b204da5658456b48b21010100e2277e52528e1a5e6117e45300e3f5f169b1712292399d065bc5167c54b8e0b5"

		// when
		registerTxChannel <- txWithoutMerklePath[:]
		processor.StartProcessRegisterTxs()

		// give blocktx time to pull all transactions from block and calculate the merkle path
		time.Sleep(200 * time.Millisecond)

		// then
		publishedTxs := getPublishedTxs(publishedTxsCh)
		tx := publishedTxs[0]
		require.Equal(t, txWithoutMerklePath[:], tx.GetTransactionHash())
		require.Equal(t, expectedMerklePath, tx.GetMerklePath())
	})
}
