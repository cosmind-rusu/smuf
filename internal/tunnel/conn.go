package tunnel

import (
	"bufio"
	"net"
)

// BufConn wraps a net.Conn con un bufio.Reader ya existente para que
// los bytes bufferizados durante el handshake no se pierdan al pasar
// la conexión a yamux.
type BufConn struct {
	net.Conn
	r *bufio.Reader
}

func NewBufConn(conn net.Conn, r *bufio.Reader) *BufConn {
	return &BufConn{Conn: conn, r: r}
}

func (c *BufConn) Read(b []byte) (int, error) {
	return c.r.Read(b)
}
