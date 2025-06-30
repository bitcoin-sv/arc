package integrationtest

import (
	"database/sql"
	"flag"
	"log"
	"os"
	"testing"

	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/ory/dockertest/v3"

	testutils "github.com/bitcoin-sv/arc/pkg/test_utils"
)

const migrationsPath = "file://../store/postgresql/migrations"

var (
	dbInfo string
	dbConn *sql.DB
)

func TestMain(m *testing.M) {
	flag.Parse()

	if testing.Short() {
		return
	}

	os.Exit(testmain(m))
}

func testmain(m *testing.M) int {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("failed to create pool: %v", err)
		return 1
	}

	port := "5437"
	resource, connStr, err := testutils.RunAndMigratePostgresql(pool, port, "blocktx", migrationsPath)
	if err != nil {
		log.Print(err)
		return 1
	}
	defer func() {
		err = pool.Purge(resource)
		if err != nil {
			log.Fatalf("failed to purge pool: %v", err)
		}
	}()

	dbInfo = connStr

	dbConn, err = sql.Open("postgres", dbInfo)
	if err != nil {
		log.Fatalf("failed to create db connection: %v", err)
		return 1
	}

	return m.Run()
}
