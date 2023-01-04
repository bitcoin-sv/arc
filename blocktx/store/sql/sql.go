package sql

import (
	"database/sql"
	"fmt"
	"log"
	"path"
	"path/filepath"

	"github.com/TAAL-GmbH/arc/blocktx/store"
	_ "github.com/lib/pq"
	"github.com/ordishs/gocore"
	_ "modernc.org/sqlite"
)

type SQL struct {
	db *sql.DB
}

func NewSQLStore(engine string) (store.Interface, error) {
	var db *sql.DB
	var err error

	var memory bool

	switch engine {
	case "postgres":
		dbHost, _ := gocore.Config().Get("dbHost", "localhost")
		dbPort, _ := gocore.Config().GetInt("dbPort", 5432)
		dbName, _ := gocore.Config().Get("dbName", "blocktx")
		dbUser, _ := gocore.Config().Get("dbUser", "blocktx")
		dbPassword, _ := gocore.Config().Get("dbPassword", "blocktx")

		dbInfo := fmt.Sprintf("user=%s password=%s dbname=%s sslmode=disable host=%s port=%d", dbUser, dbPassword, dbName, dbHost, dbPort)

		db, err = sql.Open(engine, dbInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to open postgres DB: %+v", err)
		}

		if err := createPostgresSchema(db); err != nil {
			return nil, fmt.Errorf("failed to create postgres schema: %+v", err)
		}

	case "sqlite_memory":
		memory = true
		fallthrough
	case "sqlite":
		folder, _ := gocore.Config().Get("sqliteFolder", "")

		filename, err := filepath.Abs(path.Join(folder, "block-tx.db"))
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path for sqlite DB: %+v", err)
		}

		if memory {
			filename = ":memory:"
		} else {
			// filename = fmt.Sprintf("file:%s?cache=shared&mode=rwc", filename)
			filename = fmt.Sprintf("%s?_pragma=busy_timeout=10000&_pragma=journal_mode=WAL", filename)
		}

		log.Printf("Using sqlite DB: %s", filename)

		db, err = sql.Open("sqlite", filename)
		if err != nil {
			return nil, fmt.Errorf("failed to open sqlite DB: %+v", err)
		}

		if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
			db.Close()
			return nil, fmt.Errorf("could not enable foreign keys support: %+v", err)
		}

		if _, err := db.Exec(`PRAGMA locking_mode = SHARED;`); err != nil {
			db.Close()
			return nil, fmt.Errorf("could not enable shared locking mode: %+v", err)
		}

		if err := createSqliteSchema(db); err != nil {
			return nil, fmt.Errorf("failed to create sqlite schema: %+v", err)
		}

	default:
		return nil, fmt.Errorf("unknown database engine: %s", engine)
	}

	return &SQL{
		db: db,
	}, nil
}

func createPostgresSchema(db *sql.DB) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS blocks (
		 id           BIGSERIAL PRIMARY KEY
	  ,hash         BYTEA NOT NULL
	  ,header       BYTEA NOT NULL
	  ,height       BIGINT NOT NULL
	  ,processedyn  BOOLEAN NOT NULL DEFAULT FALSE
	  ,orphanedyn   BOOLEAN NOT NULL DEFAULT FALSE
		);
	`); err != nil {
		db.Close()
		return fmt.Errorf("could not create blocks table - [%+v]", err)
	}

	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS ux_blocks_hash ON blocks (hash);`); err != nil {
		db.Close()
		return fmt.Errorf("could not create ux_blocks_hash index - [%+v]", err)
	}

	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS pux_blocks_height ON blocks(height) WHERE orphanedyn = FALSE;`); err != nil {
		db.Close()
		return fmt.Errorf("could not create pux_blocks_height index - [%+v]", err)
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS transactions (
		 id           BIGSERIAL PRIMARY KEY
	  ,hash         BYTEA NOT NULL
	  ,source       TEXT
		);
	`); err != nil {
		db.Close()
		return fmt.Errorf("could not create transactions table - [%+v]", err)
	}

	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS ux_transactions_hash ON transactions (hash);`); err != nil {
		db.Close()
		return fmt.Errorf("could not create transactions hash index - [%+v]", err)
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS block_transactions_map (
		 blockid      BIGINT NOT NULL REFERENCES blocks(id)
	  ,txid       	BIGINT NOT NULL REFERENCES transactions(id)
	  ,PRIMARY KEY (blockid, txid)
		);
	`); err != nil {
		db.Close()
		return fmt.Errorf("could not create block_transactions_map table - [%+v]", err)
	}

	return nil
}

func createSqliteSchema(db *sql.DB) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS blocks (
		 id           INTEGER PRIMARY KEY AUTOINCREMENT,
		 hash         BLOB NOT NULL
	  ,header       BLOB NOT NULL
	  ,height       BIGINT NOT NULL
	  ,processedyn  BOOLEAN NOT NULL DEFAULT FALSE
	  ,orphanedyn   BOOLEAN NOT NULL DEFAULT FALSE
	 	);
	`); err != nil {
		db.Close()
		return fmt.Errorf("could not create blocks table - [%+v]", err)
	}

	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS ux_blocks_hash ON blocks (hash);`); err != nil {
		db.Close()
		return fmt.Errorf("could not create ux_blocks_hash index - [%+v]", err)
	}

	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS pux_blocks_height ON blocks(height) WHERE orphanedyn = FALSE;`); err != nil {
		db.Close()
		return fmt.Errorf("could not create pux_blocks_height index - [%+v]", err)
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS transactions (
		 id           INTEGER PRIMARY KEY AUTOINCREMENT,
		 hash         BLOB NOT NULL
	  ,source				TEXT
	 	);
	`); err != nil {
		db.Close()
		return fmt.Errorf("could not create transactions table - [%+v]", err)
	}

	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS ux_transactions_hash ON transactions (hash);`); err != nil {
		db.Close()
		return fmt.Errorf("could not create transactions hash index - [%+v]", err)
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS block_transactions_map (
		 blockid      INTEGER NOT NULL
		,txid         INTEGER NOT NULL
		,FOREIGN KEY (blockid) REFERENCES blocks(id)
		,FOREIGN KEY (txid) REFERENCES transactions(id)
	  ,PRIMARY KEY (blockid, txid)
	  );
	`); err != nil {
		db.Close()
		return fmt.Errorf("could not create block_transactions_map table - [%+v]", err)
	}

	return nil
}
