package testutils

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"

	"github.com/golang-migrate/migrate/v4"
	migratepostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func MigrateUp(table, path, dbInfo string) error {
	dbConn, err := sql.Open("postgres", dbInfo)
	if err != nil {
		return fmt.Errorf("failed to create db connection: %v", err)
	}
	defer func() {
		_ = dbConn.Close()
	}()

	if err = Retry(dbConn.Ping); err != nil {
		return fmt.Errorf("failed to connect to docker: %s", err)
	}

	driver, err := migratepostgres.WithInstance(dbConn, &migratepostgres.Config{
		MigrationsTable: table,
	})
	if err != nil {
		return fmt.Errorf("failed to create driver: %v", err)
	}

	migrations, err := migrate.NewWithDatabaseInstance(
		path,
		"postgres",
		driver)
	if err != nil {
		return fmt.Errorf("failed to initialize migrate instance: %v", err)
	}

	err = migrations.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to initialize migrate instance: %v", err)
	}

	return nil
}

func Retry(op func() error) error {
	bo := backoff.NewExponentialBackOff()
	bo.MaxInterval = time.Second * 5
	bo.MaxElapsedTime = time.Minute
	if err := backoff.Retry(op, bo); err != nil {
		if bo.NextBackOff() == backoff.Stop {
			return fmt.Errorf("reached retry deadline: %w", err)
		}

		return err
	}

	return nil
}
