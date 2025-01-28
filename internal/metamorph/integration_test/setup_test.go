package integrationtest

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/go-redis/redis/v8"
	"github.com/ory/dockertest/v3"

	"github.com/bitcoin-sv/arc/pkg/test_utils"
)

const (
	postgresqlPort = "5435"
	redisPort      = "6380"
	migrationsPath = "file://../store/postgresql/migrations"
)

var (
	dbInfo string
	dbConn *sql.DB

	redisClient *redis.Client
)

func TestMain(m *testing.M) {
	os.Exit(testmain(m))
}

func testmain(m *testing.M) int {
	// prepare postgresql
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Printf("failed to create pool: %v", err)
		return 1
	}

	resourcePsql, connStr, err := testutils.RunAndMigratePostgresql(pool, postgresqlPort, "metamorph", migrationsPath)
	if err != nil {
		log.Print(err)
		return 1
	}
	defer func() {
		err = pool.Purge(resourcePsql)
		if err != nil {
			log.Fatalf("failed to purge pool: %v", err)
		}
	}()

	dbInfo = connStr

	dbConn, err = sql.Open("postgres", dbInfo)
	if err != nil {
		log.Printf("failed to create db connection: %v", err)
		return 1
	}

	// prepare redis
	resourceRedis, resourcePort, err := testutils.RunRedis(pool, redisPort, "mtm-txs-cache")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		err = pool.Purge(resourceRedis)
		if err != nil {
			log.Fatalf("failed to purge pool: %v", err)
		}
	}()

	redisClient = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("localhost:%s", resourcePort),
		Password: "",
		DB:       1,
	})

	_, err = redisClient.Ping(context.Background()).Result()
	if err != nil {
		log.Fatalln(err)
	}

	return m.Run()
}
