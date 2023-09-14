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
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	_ "modernc.org/sqlite"
)

const (
	postgresEngine     = "postgres"
	sqliteEngine       = "sqlite"
	sqliteMemoryEngine = "sqlite_memory"
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

	logLevel := viper.GetString("logLevel")
	if logLevel == "" {
		logLevel = "INFO"
	}
	logger := gocore.Log("btsql", gocore.NewLogLevelFromString(logLevel))

	switch engine {
	case postgresEngine:

		dbName := viper.GetString("blocktx.db.postgres.name")
		if dbName == "" {
			return nil, errors.Errorf("setting blocktx.db.postgres.name not found")
		}

		dbPassword := viper.GetString("blocktx.db.postgres.password")
		if dbPassword == "" {
			return nil, errors.Errorf("setting blocktx.db.postgres.password not found")
		}

		dbUser := viper.GetString("blocktx.db.postgres.user")
		if dbUser == "" {
			return nil, errors.Errorf("setting blocktx.db.postgres.user not found")
		}

		dbHost := viper.GetString("blocktx.db.postgres.host")
		if dbHost == "" {
			return nil, errors.Errorf("setting blocktx.db.postgres.host not found")
		}

		dbPort := viper.GetInt("blocktx.db.postgres.port")
		if dbPort == 0 {
			return nil, errors.Errorf("setting blocktx.db.postgres.port not found")
		}

		dbInfo := fmt.Sprintf("user=%s password=%s dbname=%s sslmode=disable host=%s port=%d", dbUser, dbPassword, dbName, dbHost, dbPort)

		db, err = sql.Open(engine, dbInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to open postgres DB: %+v", err)
		}

		idleConns := viper.GetInt("blocktx.db.postgres.maxIdleConns")
		if idleConns == 0 {
			return nil, errors.Errorf("setting blocktx.db.postgres.maxIdleConns not found")
		}

		db.SetMaxIdleConns(idleConns)
		maxOpenConns := viper.GetInt("blocktx.db.postgres.maxOpenConns")
		if maxOpenConns == 0 {
			return nil, errors.Errorf("setting blocktx.db.postgres.maxOpenConns not found")
		}

		db.SetMaxOpenConns(maxOpenConns)

		if err := createPostgresSchema(db); err != nil {
			return nil, fmt.Errorf("failed to create postgres schema: %+v", err)
		}

	case sqliteMemoryEngine:
		memory = true
		fallthrough
	case sqliteEngine:
		var filename string
		if memory {
			filename = fmt.Sprintf("file:%s?mode=memory&cache=shared", random.String(16))
		} else {
			folder := viper.GetString("dataFolder")
			if folder == "" {
				return nil, errors.Errorf("setting dataFolder not found")
			}
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
		,merkle_path  TEXT
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
		return fmt.Errorf("could not create function table reverse_bytes_iter() [%+v]", err)
	}

	if _, err := db.Exec(`
		CREATE OR REPLACE FUNCTION reverse_bytes(bytes bytea) RETURNS bytea AS
		'SELECT reverse_bytes_iter(bytes, octet_length(bytes)-1, octet_length(bytes)/2, 0)'
		LANGUAGE SQL IMMUTABLE;
	`); err != nil {
		db.Close()
		return fmt.Errorf("could not create function reverse_bytes() - [%+v]", err)
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
	  ,merkle_path			TEXT
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
