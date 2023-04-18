package sql

import (
	"database/sql"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/bitcoin-sv/arc/blocktx/store"
	"github.com/labstack/gommon/random"
	_ "github.com/lib/pq"
	"github.com/ordishs/gocore"
	_ "modernc.org/sqlite"
)

type SQL struct {
	db     *sql.DB
	engine string
}

func init() {
	gocore.NewStat("blocktx")
}

func New(engine string) (store.Interface, error) {
	var db *sql.DB
	var err error

	var memory bool

	logLevel, _ := gocore.Config().Get("logLevel")
	logger := gocore.Log("btsql", gocore.NewLogLevelFromString(logLevel))

	switch engine {
	case "postgres":
		dbHost, _ := gocore.Config().Get("blocktx_dbHost", "localhost")
		dbPort, _ := gocore.Config().GetInt("blocktx_dbPort", 5432)
		dbName, _ := gocore.Config().Get("blocktx_dbName", "blocktx")
		dbUser, _ := gocore.Config().Get("blocktx_dbUser", "arc")
		dbPassword, _ := gocore.Config().Get("blocktx_dbPassword", "arc")

		dbInfo := fmt.Sprintf("user=%s password=%s dbname=%s sslmode=disable host=%s port=%d", dbUser, dbPassword, dbName, dbHost, dbPort)

		db, err = sql.Open(engine, dbInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to open postgres DB: %+v", err)
		}

		idleConns, _ := gocore.Config().GetInt("blocktx_postgresMaxIdleConns", 10)
		db.SetMaxIdleConns(idleConns)
		maxOpenConns, _ := gocore.Config().GetInt("blocktx_postgresMaxOpenConns", 80)
		db.SetMaxOpenConns(maxOpenConns)

		if err := createPostgresSchema(db); err != nil {
			return nil, fmt.Errorf("failed to create postgres schema: %+v", err)
		}

	case "sqlite_memory":
		memory = true
		fallthrough
	case "sqlite":
		var filename string
		if memory {
			filename = fmt.Sprintf("file:%s?mode=memory&cache=shared", random.String(16))
		} else {
			folder, _ := gocore.Config().Get("dataFolder", "data")
			if err = os.MkdirAll(folder, 0755); err != nil {
				return nil, fmt.Errorf("failed to create data folder %s: %+v", folder, err)
			}

			filename, err = filepath.Abs(path.Join(folder, "blocktx.db"))
			if err != nil {
				return nil, fmt.Errorf("failed to get absolute path for sqlite DB: %+v", err)
			}

			// filename = fmt.Sprintf("file:%s?cache=shared&mode=rwc", filename)
			filename = fmt.Sprintf("%s?cache=shared&_pragma=busy_timeout=10000&_pragma=journal_mode=WAL", filename)
		}

		logger.Infof("Using sqlite DB: %s", filename)

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
		db:     db,
		engine: engine,
	}, nil
}

func (s *SQL) Close() error {
	return s.db.Close()
}

func createPostgresSchema(db *sql.DB) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS blocks (
		 inserted_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		,id           BIGSERIAL PRIMARY KEY
	  ,hash         BYTEA NOT NULL
	  ,prevhash     BYTEA NOT NULL
	  ,merkleroot   BYTEA NOT NULL
	  ,height       BIGINT NOT NULL
	  ,processed_at TIMESTAMPTZ
		,size					BIGINT
		,tx_count			BIGINT
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
		,pos					BIGINT NOT NULL
	  ,PRIMARY KEY (blockid, txid)
		);
	`); err != nil {
		db.Close()
		return fmt.Errorf("could not create block_transactions_map table - [%+v]", err)
	}

	if _, err := db.Exec(`
	CREATE OR REPLACE FUNCTION reverse_bytes_iter(bytes bytea, length int, midpoint int, index int)
	RETURNS bytea AS
	$$
  SELECT CASE WHEN index >= midpoint THEN bytes ELSE
    reverse_bytes_iter(
      set_byte(
        set_byte(bytes, index, get_byte(bytes, length-index)),
        length-index, get_byte(bytes, index)
      ),
      length, midpoint, index + 1
    )
  END;
	$$ LANGUAGE SQL IMMUTABLE;
`); err != nil {
		db.Close()
		return fmt.Errorf("could not create block_transactions_map table - [%+v]", err)
	}

	if _, err := db.Exec(`
		CREATE OR REPLACE FUNCTION reverse_bytes(bytes bytea) RETURNS bytea AS
		'SELECT reverse_bytes_iter(bytes, octet_length(bytes)-1, octet_length(bytes)/2, 0)'
		LANGUAGE SQL IMMUTABLE;
	`); err != nil {
		db.Close()
		return fmt.Errorf("could not create block_transactions_map table - [%+v]", err)
	}

	return nil
}

func createSqliteSchema(db *sql.DB) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS blocks (
		 inserted_at  TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		,id           INTEGER PRIMARY KEY AUTOINCREMENT
		,hash         BLOB NOT NULL
	  ,prevhash     BLOB NOT NULL
		,merkleroot   BLOB NOT NULL
	  ,height       BIGINT NOT NULL
	  ,processed_at TEXT
		,size					BIGINT
		,tx_count			BIGINT
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
		,pos					INTEGER NOT NULL
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
