package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/labstack/gommon/random"
	"github.com/opentracing/opentracing-go"
	"github.com/ordishs/gocore"
	_ "modernc.org/sqlite"
)

type SqLite struct {
	db *sql.DB
}

func New(memory bool, folder string) (*SqLite, error) {
	var db *sql.DB

	var err error
	var filename string

	if memory {
		filename = fmt.Sprintf("file:%s?mode=memory&cache=shared", random.String(16))
	} else {

		if err = os.MkdirAll(folder, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create data folder %s: %+v", folder, err)
		}

		filename, err = filepath.Abs(path.Join(folder, "blocktx.db"))
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path for sqlite DB: %+v", err)
		}

		filename = fmt.Sprintf("%s?cache=shared&_pragma=busy_timeout=10000&_pragma=journal_mode=WAL", filename)
	}

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

	return &SqLite{
		db: db,
	}, nil
}

func (s *SqLite) Ping(ctx context.Context) error {
	startNanos := time.Now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("Ping").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:Ping")
	defer span.Finish()

	_, err := s.db.QueryContext(ctx, "SELECT 1;")
	if err != nil {
		return err
	}

	return nil
}

func (s *SqLite) Close() error {
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
