package sql

import (
	"database/sql"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/bitcoin-sv/arc/blocktx/store"
	"github.com/bitcoin-sv/arc/dbconn"
	"github.com/labstack/gommon/random"
	_ "github.com/lib/pq"
	"github.com/ordishs/gocore"
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

// NewPostgresStore postgres storage that accepts connection parameters and returns connection or an error.
func NewPostgresStore(params dbconn.DBConnectionParams) (store.Interface, error) {
	db, err := sql.Open("postgres", params.String())
	if err != nil {
		return nil, fmt.Errorf("unable to connect to db due to %s", err)
	}

	return &SQL{
		db:     db,
		engine: postgresEngine,
	}, nil
}

func New(engine string) (*SQL, error) {
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
			return nil, fmt.Errorf("setting blocktx.db.postgres.name not found")
		}

		dbPassword := viper.GetString("blocktx.db.postgres.password")
		if dbPassword == "" {
			return nil, fmt.Errorf("setting blocktx.db.postgres.password not found")
		}

		dbUser := viper.GetString("blocktx.db.postgres.user")
		if dbUser == "" {
			return nil, fmt.Errorf("setting blocktx.db.postgres.user not found")
		}

		dbHost := viper.GetString("blocktx.db.postgres.host")
		if dbHost == "" {
			return nil, fmt.Errorf("setting blocktx.db.postgres.host not found")
		}

		dbPort := viper.GetInt("blocktx.db.postgres.port")
		if dbPort == 0 {
			return nil, fmt.Errorf("setting blocktx.db.postgres.port not found")
		}

		sslMode := viper.GetString("blocktx.db.postgres.sslMode")
		if sslMode == "" {
			return nil, fmt.Errorf("setting blocktx.db.postgres.sslMode not found")
		}

		dbInfo := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=%d sslmode=%s", dbUser, dbPassword, dbName, dbHost, dbPort, sslMode)

		db, err = sql.Open(engine, dbInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to open postgres DB: %+v", err)
		}

		idleConns := viper.GetInt("blocktx.db.postgres.maxIdleConns")
		if idleConns == 0 {
			return nil, fmt.Errorf("setting blocktx.db.postgres.maxIdleConns not found")
		}

		db.SetMaxIdleConns(idleConns)
		maxOpenConns := viper.GetInt("blocktx.db.postgres.maxOpenConns")
		if maxOpenConns == 0 {
			return nil, fmt.Errorf("setting blocktx.db.postgres.maxOpenConns not found")
		}

		db.SetMaxOpenConns(maxOpenConns)

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
				return nil, fmt.Errorf("setting dataFolder not found")
			}
			if err = os.MkdirAll(folder, 0o755); err != nil {
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
	  ,merkle_path			TEXT DEFAULT('')
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

	if _, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS primary_blocktx (
		host_name TEXT PRIMARY KEY,
		primary_until TIMESTAMP
	);
	`); err != nil {
		db.Close()
		return fmt.Errorf("could not create primary_blocktx table - [%+v]", err)
	}
	return nil
}
