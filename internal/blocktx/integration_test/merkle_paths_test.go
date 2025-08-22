package integrationtest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	testutils "github.com/bitcoin-sv/arc/pkg/test_utils"
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

		registeredTxHash := testutils.RevHexDecodeString(t, "ff2ea4f998a94d5128ac7663824b2331cc7f95ca60611103f49163d4a4eb547c")
		expectedMerklePath := "fe175b1900040200027c54eba4d46391f403116160ca957fcc31234b826376ac28514da998f9a42eff01008308c059d119b87a9520a9fd3d765f8b74f726e96c77a1a6623d71796cc8c207010100333ff2bf5a4128823576d192e3e7f4a78e287b1acd0716127d28a890b7fd37930101006b473b8dc8557542d316becceae316b142f21b0ba4d08d17da55cf4b7b6817c90101009ea3342e55faab48aaa301e3f1935428b0bba92931e0ad24081d2ebae70bc160"

		// when
		registerTxChannel <- registeredTxHash[:]
		processor.StartProcessRegisterTxs()

		// give blocktx time to pull all transactions from block and calculate the merkle path
		time.Sleep(200 * time.Millisecond)

		// then
		publishedTxs := getPublishedTxs(publishedTxsCh)
		require.Len(t, publishedTxs, 1)
		tx := publishedTxs[0]
		require.Equal(t, registeredTxHash, tx.GetTransactionHash())
		require.Equal(t, expectedMerklePath, tx.GetMerklePath())
	})
}
