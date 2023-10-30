package dbconn

import "fmt"

type DBConnectionParams struct {
	Host     string
	Port     int
	Username string
	Password string
	DBName   string
	Scheme   string
}

func (p DBConnectionParams) String() string {
	return fmt.Sprintf("%s://%s:%s@%s:%d/%s?sslmode=disable", p.Scheme, p.Username, p.Password, p.Host, p.Port, p.DBName)
}
