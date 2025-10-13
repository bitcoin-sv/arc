package integrationtest

// Components of this test:
// 		ZMQ - mocked
// 		Metamorph Processor
// 		Postgresql Store - running on docker
// Flow of this test:
// 		1. A hardcoded double spend message is being sent to mocked ZMQ
// 		2. ZMQ parses that message and sends updates to processor
// 		3. Processor handles the update and runs a proper store update function (UpdateDoubleSpend)
// 		4. Both transactions (one already SEEN_ON_NETWORK and the new one) are properly updated to DOUBLE_SPEND_ATTEMPTED status
// 		5. Their statuses are verified by getting the transactions data from the postgresql store
// 		6. MinedTxBlock is being sent to minedTxChannel of metamorph processor. This simulates a msg from blocktx about mined transaction
// 		7. This transaction is processed and status updated to MINED. The status of second tx is updated to REJECTED
// 		8. Statuses of txs are verified by getting the transactions data from the postgresql store

import (
	"context"
	"database/sql"
	"log/slog"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-bt/v2/chainhash"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/cache"
	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/internal/metamorph/bcnet"
	"github.com/bitcoin-sv/arc/internal/metamorph/bcnet/metamorph_p2p"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/mocks"
	"github.com/bitcoin-sv/arc/internal/metamorph/store/postgresql"
	testutils "github.com/bitcoin-sv/arc/pkg/test_utils"
)

const (
	zmqTopic                = "invalidtx"
	msgDoubleSpendAttempted = "7b2266726f6d426c6f636b223a2066616c73652c22736f75726365223a2022703270222c2261646472657373223a20226e6f6465323a3138333333222c226e6f64654964223a20312c2274786964223a202238653735616531306638366438613433303434613534633363353764363630643230636462373465323333626534623563393062613735326562646337653838222c2273697a65223a203139312c22686578223a202230313030303030303031313134386239653931646336383232313635306539363861366164613863313531373135656135373864623130376336623563333362363762376636376630323030303030303030366134373330343430323230313863396166396334626634653736383932376263363335363233623434383362656261656334343433396165613838356363666430363163373731636435613032323034613839626531333534613038613539643466316636323235343937366532373466316333333334383334373137363462623936633565393837626539663365343132313033303830373637393438326663343533323461386133326166643832333730646337316365383966373936376536636635646139646430356330366665356137616666666666666666303130613030303030303030303030303030313937366139313434613037363038353032653464646131363662333830343130613633663066653962383830666532383861633030303030303030222c226973496e76616c6964223a20747275652c22697356616c69646174696f6e4572726f72223a2066616c73652c2269734d697373696e67496e70757473223a2066616c73652c226973446f75626c655370656e644465746563746564223a2066616c73652c2269734d656d706f6f6c436f6e666c6963744465746563746564223a20747275652c2269734e6f6e46696e616c223a2066616c73652c22697356616c69646174696f6e54696d656f75744578636565646564223a2066616c73652c2269735374616e646172645478223a20747275652c2272656a656374696f6e436f6465223a203235382c2272656a656374696f6e526561736f6e223a202274786e2d6d656d706f6f6c2d636f6e666c696374222c22636f6c6c6964656457697468223a205b7b2274786964223a202264363461646663653662313035646336626466343735343934393235626630363830326134316130353832353836663333633262313664353337613062376236222c2273697a65223a203139312c22686578223a202230313030303030303031313134386239653931646336383232313635306539363861366164613863313531373135656135373864623130376336623563333362363762376636376630323030303030303030366134373330343430323230376361326162353332623936303130333362316464636138303838353433396366343433666264663262616463656637303964383930616434373661346162353032323032653730666565353935313462313763353635336138313834643730646232646363643062613339623731663730643239386231643939313764333837396663343132313033303830373637393438326663343533323461386133326166643832333730646337316365383966373936376536636635646139646430356330366665356137616666666666666666303130613030303030303030303030303030313937366139313435313335306233653933363037613437616136623161653964343937616336656135366130623132383861633030303030303030227d5d2c2272656a656374696f6e54696d65223a2022323032342d30372d32355431313a30313a35365a227d"
)

// msgDoubleSpendAttempted contains these hashes
var hashes = []string{"8e75ae10f86d8a43044a54c3c57d660d20cdb74e233be4b5c90ba752ebdc7e88", "d64adfce6b105dc6bdf475494925bf06802a41a0582586f33c2b16d537a0b7b6"}

func pruneTables(t *testing.T, db *sql.DB) {
	testutils.PruneTables(t, db, "metamorph.Transactions")
}

func TestDoubleSpendDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	t.Run("double spend detection", func(t *testing.T) {
		defer pruneTables(t, dbConn)
		testutils.LoadFixtures(t, dbConn, "fixtures/double_spend_detection")

		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

		statusMessageChannel := make(chan *metamorph_p2p.TxStatusMessage, 10)
		minedTxChannel := make(chan *blocktx_api.TransactionBlocks, 10)

		mockedZMQ := &mocks.ZMQIMock{
			SubscribeFunc: func(s string, stringsCh chan []string) error {
				if s != zmqTopic {
					return nil
				}
				event := make([]string, 0)
				event = append(event, zmqTopic)
				event = append(event, msgDoubleSpendAttempted)
				event = append(event, "2459")
				stringsCh <- event
				return nil
			},
		}

		metamorphStore, err := postgresql.New(dbInfo, "double-spend-integration-test", 10, 80)
		require.NoError(t, err)
		defer metamorphStore.Close(context.Background())

		cStore := cache.NewMemoryStore()
		for _, h := range hashes {
			err := cStore.Set(h, []byte("1"), 10*time.Minute) // make sure txs are registered
			require.NoError(t, err)
		}

		pm := &bcnet.Mediator{}

		processor, err := metamorph.NewProcessor(metamorphStore, cStore, pm, statusMessageChannel,
			metamorph.WithMinedTxsChan(minedTxChannel),
			metamorph.WithNow(func() time.Time { return time.Date(2023, 10, 1, 13, 0, 0, 0, time.UTC) }),
			metamorph.WithStatusUpdatesInterval(200*time.Millisecond),
			metamorph.WithProcessStatusUpdatesBatchSize(3))
		require.NoError(t, err)
		defer processor.Shutdown()

		processor.StartProcessStatusUpdatesInStorage()
		processor.StartSendStatusUpdate()
		processor.StartProcessMinedCallbacks()

		zmqURL, err := url.Parse("https://some-url.com")
		require.NoError(t, err)

		zmq, err := metamorph.NewZMQ(zmqURL, statusMessageChannel, mockedZMQ, logger)
		require.NoError(t, err)
		cleanup, err := zmq.Start()
		require.NoError(t, err)
		defer cleanup()

		// give metamorph time to parse ZMQ message
		time.Sleep(500 * time.Millisecond)

		for i, hashStr := range hashes {
			h, err := chainhash.NewHashFromStr(hashStr)
			require.NoError(t, err)

			txData, err := metamorphStore.Get(context.Background(), h[:])
			require.NoError(t, err)

			require.Equal(t, metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED, txData.Status)
			require.Equal(t, hashes[i], txData.Hash.String())
		}

		minedTxHash, err := chainhash.NewHashFromStr(hashes[0])
		require.NoError(t, err)

		minedMsg := &blocktx_api.TransactionBlocks{
			TransactionBlocks: []*blocktx_api.TransactionBlock{
				{
					BlockHash:       testutils.RevChainhash(t, "000000000000000003a450dba927fa46fe26079c32244a2f70301de574d84269")[:],
					BlockHeight:     888888,
					TransactionHash: minedTxHash[:],
					MerklePath:      "merkle-path-1",
				},
			},
		}

		minedTxChannel <- minedMsg

		// give metamorph time to parse mined msg
		time.Sleep(1000 * time.Millisecond)

		// verify that the 1st hash is mined
		minedTxData, err := metamorphStore.Get(context.Background(), minedTxHash[:])
		require.NoError(t, err)

		require.Equal(t, metamorph_api.Status_MINED, minedTxData.Status)

		// verify that the 2nd hash is rejected
		rejectedHash, err := chainhash.NewHashFromStr(hashes[1])
		require.NoError(t, err)

		rejectedTxData, err := metamorphStore.Get(context.Background(), rejectedHash[:])
		require.NoError(t, err)

		require.Equal(t, metamorph_api.Status_REJECTED, rejectedTxData.Status)
		require.Equal(t, "double spend attempted", rejectedTxData.RejectReason)
	})
}
