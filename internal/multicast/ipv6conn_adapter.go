package multicast

import (
	"io"
	"net"

	"golang.org/x/net/ipv6"
)

var (
	_ io.Reader = (*ipv6ConnAdapter)(nil)
	_ io.Writer = (*ipv6ConnAdapter)(nil)
)

type ipv6ConnAdapter struct {
	Conn *ipv6.PacketConn
	dst  *net.UDPAddr
}

func (c *ipv6ConnAdapter) Read(b []byte) (n int, err error) {
	n, _, _, err = c.Conn.ReadFrom(b)
	return
}

func (c *ipv6ConnAdapter) Write(b []byte) (n int, err error) {
	return c.Conn.WriteTo(b, nil, c.dst)
}
