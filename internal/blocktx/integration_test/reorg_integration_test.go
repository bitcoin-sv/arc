package integrationtest

// Components of this test:
// 		Postgresql Store - running on docker
// 		Blocktx Processor
// 		PeerHandler - mocked
//
// Flow of this test:
// 		1. A list of blocks from height 822014 to 822017 is added to db from fixtures
// 		2. A hardcoded msg with competing block at height 822015 is being send through the mocked PeerHandler
// 		3. This block has a chainwork lower than the current tip of chain - becomes STALE
// 		4. Next competing block, at height 822016 is being send through the mocked PeerHandler
// 		5. This block has a greater chainwork than the current tip of longest chain - it becomes LONGEST despite not being the highest
//
// Future implementation in next tasks:
// 		- Verify if reorg was performed correctly, if previous blocks have updated statuses
// 		- Include metamorph in this test and verify that transactions statuses are properly updated

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store/postgresql"
	"github.com/golang-migrate/migrate/v4"
	migratepostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/require"
)

const (
	zmqTopic                = "invalidtx"
	msgDoubleSpendAttempted = "7b2266726f6d426c6f636b223a2066616c73652c22736f75726365223a2022703270222c2261646472657373223a20226e6f6465323a3138333333222c226e6f64654964223a20312c2274786964223a202238653735616531306638366438613433303434613534633363353764363630643230636462373465323333626534623563393062613735326562646337653838222c2273697a65223a203139312c22686578223a202230313030303030303031313134386239653931646336383232313635306539363861366164613863313531373135656135373864623130376336623563333362363762376636376630323030303030303030366134373330343430323230313863396166396334626634653736383932376263363335363233623434383362656261656334343433396165613838356363666430363163373731636435613032323034613839626531333534613038613539643466316636323235343937366532373466316333333334383334373137363462623936633565393837626539663365343132313033303830373637393438326663343533323461386133326166643832333730646337316365383966373936376536636635646139646430356330366665356137616666666666666666303130613030303030303030303030303030313937366139313434613037363038353032653464646131363662333830343130613633663066653962383830666532383861633030303030303030222c226973496e76616c6964223a20747275652c22697356616c69646174696f6e4572726f72223a2066616c73652c2269734d697373696e67496e70757473223a2066616c73652c226973446f75626c655370656e644465746563746564223a2066616c73652c2269734d656d706f6f6c436f6e666c6963744465746563746564223a20747275652c2269734e6f6e46696e616c223a2066616c73652c22697356616c69646174696f6e54696d656f75744578636565646564223a2066616c73652c2269735374616e646172645478223a20747275652c2272656a656374696f6e436f6465223a203235382c2272656a656374696f6e526561736f6e223a202274786e2d6d656d706f6f6c2d636f6e666c696374222c22636f6c6c6964656457697468223a205b7b2274786964223a202264363461646663653662313035646336626466343735343934393235626630363830326134316130353832353836663333633262313664353337613062376236222c2273697a65223a203139312c22686578223a202230313030303030303031313134386239653931646336383232313635306539363861366164613863313531373135656135373864623130376336623563333362363762376636376630323030303030303030366134373330343430323230376361326162353332623936303130333362316464636138303838353433396366343433666264663262616463656637303964383930616434373661346162353032323032653730666565353935313462313763353635336138313834643730646232646363643062613339623731663730643239386231643939313764333837396663343132313033303830373637393438326663343533323461386133326166643832333730646337316365383966373936376536636635646139646430356330366665356137616666666666666666303130613030303030303030303030303030313937366139313435313335306233653933363037613437616136623161653964343937616336656135366130623132383861633030303030303030227d5d2c2272656a656374696f6e54696d65223a2022323032342d30372d32355431313a30313a35365a227d"
)

// msgDoubleSpendAttempted contains these hashes
var hashes = []string{"8e75ae10f86d8a43044a54c3c57d660d20cdb74e233be4b5c90ba752ebdc7e88", "d64adfce6b105dc6bdf475494925bf06802a41a0582586f33c2b16d537a0b7b6"}

const (
	postgresPort   = "5432"
	migrationsPath = "file://../store/postgresql/migrations"
	dbName         = "main_test"
	dbUsername     = "arcuser"
	dbPassword     = "arcpass"
)

var (
	dbInfo string
	dbConn *sql.DB
)

func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("failed to create pool: %v", err)
	}

	port := "5436"
	opts := dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "15.4",
		Env: []string{
			fmt.Sprintf("POSTGRES_PASSWORD=%s", dbPassword),
			fmt.Sprintf("POSTGRES_USER=%s", dbUsername),
			fmt.Sprintf("POSTGRES_DB=%s", dbName),
			"listen_addresses = '*'",
		},
		ExposedPorts: []string{"5432"},
		PortBindings: map[docker.Port][]docker.PortBinding{
			postgresPort: {
				{HostIP: "0.0.0.0", HostPort: port},
			},
		},
	}

	resource, err := pool.RunWithOptions(&opts, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{
			Name: "no",
		}
		config.Tmpfs = map[string]string{
			"/var/lib/postgresql/data": "",
		}
	})
	if err != nil {
		log.Fatalf("failed to create resource: %v", err)
	}

	hostPort := resource.GetPort("5432/tcp")

	dbInfo = fmt.Sprintf("host=localhost port=%s user=%s password=%s dbname=%s sslmode=disable", hostPort, dbUsername, dbPassword, dbName)
	dbConn, err = sql.Open("postgres", dbInfo)
	if err != nil {
		log.Fatalf("failed to create db connection: %v", err)
	}
	err = pool.Retry(func() error {
		return dbConn.Ping()
	})
	if err != nil {
		log.Fatalf("failed to connect to docker: %s", err)
	}

	driver, err := migratepostgres.WithInstance(dbConn, &migratepostgres.Config{
		MigrationsTable: "metamorph",
	})
	if err != nil {
		log.Fatalf("failed to create driver: %v", err)
	}

	migrations, err := migrate.NewWithDatabaseInstance(migrationsPath, "postgres", driver)
	if err != nil {
		log.Fatalf("failed to initialize migrate instance: %v", err)
	}
	err = migrations.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatalf("failed to initialize migrate instance: %v", err)
	}

	code := m.Run()

	err = pool.Purge(resource)
	if err != nil {
		log.Fatalf("failed to purge pool: %v", err)
	}

	os.Exit(code)
}

func TestReorg(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	defer require.NoError(t, pruneTables(dbConn))
	require.NoError(t, loadFixtures(dbConn, "fixtures"))

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	var blockRequestCh chan blocktx.BlockRequest = nil
	blockProcessCh := make(chan *p2p.BlockMessage, 10)

	blocktxStore, err := postgresql.New(dbInfo, 10, 80)
	require.NoError(t, err)

	peerHandler := blocktx.NewPeerHandler(logger, blockRequestCh, blockProcessCh)
	processor, err := blocktx.NewProcessor(logger, blocktxStore, blockRequestCh, blockProcessCh)
	require.NoError(t, err)

	processor.StartBlockProcessing()

	prevBlockHash := revChainhash(t, "f97e20396f02ab990ed31b9aec70c240f48b7e5ea239aa050000000000000000")
	txHash, err := chainhash.NewHashFromStr("be181e91217d5f802f695e52144078f8dfbe51b8a815c3d6fb48c0d853ec683b")
	require.NoError(t, err)
	merkleRoot, err := chainhash.NewHashFromStr("be181e91217d5f802f695e52144078f8dfbe51b8a815c3d6fb48c0d853ec683b")
	require.NoError(t, err)

	// should become STALE
	blockMessage := &p2p.BlockMessage{
		Header: &wire.BlockHeader{
			Version:    541065216,
			PrevBlock:  *prevBlockHash, // block with status LONGEST at height 822014
			MerkleRoot: *merkleRoot,
			Bits:       0x1d00ffff, // chainwork: "4295032833" lower than the competing block
		},
		Height:            uint64(822015), // competing block already exists at this height
		TransactionHashes: []*chainhash.Hash{txHash},
	}

	peerHandler.HandleBlock(blockMessage, nil)
	// Allow DB to process the block
	time.Sleep(200 * time.Millisecond)

	blockHash := blockMessage.Header.BlockHash()

	block, err := blocktxStore.GetBlock(context.Background(), &blockHash)
	require.NoError(t, err)
	require.Equal(t, uint64(822015), block.Height)
	require.Equal(t, blocktx_api.Status_STALE, block.Status)

	// should become LONGEST
	blockMessage = &p2p.BlockMessage{
		Header: &wire.BlockHeader{
			Version:    541065216,
			PrevBlock:  blockHash, // block with status STALE at height 822015
			MerkleRoot: *merkleRoot,
			Bits:       0x1a05db8b, // chainwork: "12301577519373468" higher than the competing block
		},
		Height:            uint64(822016), // competing block already exists at this height
		TransactionHashes: []*chainhash.Hash{txHash},
	}

	peerHandler.HandleBlock(blockMessage, nil)
	// Allow DB to process the block
	time.Sleep(200 * time.Millisecond)

	blockHash = blockMessage.Header.BlockHash()

	block, err = blocktxStore.GetBlock(context.Background(), &blockHash)
	require.NoError(t, err)
	require.Equal(t, uint64(822016), block.Height)
	require.Equal(t, blocktx_api.Status_LONGEST, block.Status)
}
