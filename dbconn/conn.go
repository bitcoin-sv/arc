package dbconn

import "fmt"

type DBConnectionParams struct {
	host     string
	port     int
	username string
	password string
	dBName   string
	scheme   string
	sslMode  string
}

func New(host string, port int, username string, password string, dBName string, scheme string, sslMode string) DBConnectionParams {
	return DBConnectionParams{
		host:     host,
		port:     port,
		username: username,
		password: password,
		dBName:   dBName,
		scheme:   scheme,
		sslMode:  sslMode,
	}
}

func (p DBConnectionParams) String() string {
	return fmt.Sprintf("%s://%s:%s@%s:%d/%s?sslmode=%s", p.scheme, p.username, p.password, p.host, p.port, p.dBName, p.sslMode)
}

func (p DBConnectionParams) Scheme() string {
	return p.scheme
}
