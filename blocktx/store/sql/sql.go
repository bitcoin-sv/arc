package sql

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
	"github.com/ordishs/gocore"
)

type SQL struct {
	db *sql.DB
}

func New(engine, host, user, password, database string, port int) (store.Interface, error) {
	dbHost, _ := gocore.Config().Get("dbHost", "localhost")
	dbPort, _ := gocore.Config().GetInt("dbPort", 5432)
	dbName, _ := gocore.Config().Get("dbName", "blocktx")
	dbUser, _ := gocore.Config().Get("dbUser", "blocktx")
	dbPassword, _ := gocore.Config().Get("dbPassword", "blocktx")

	dbInfo := fmt.Sprintf("user=%s password=%s dbname=%s sslmode=disable host=%s port=%d", dbUser, dbPassword, dbName, dbHost, dbPort)

	dbConn, err := sql.Open(engine, dbInfo)
	if err != nil {
		return nil, err
	}

	return &SQL{
		db: dbConn,
	}, nil
}
