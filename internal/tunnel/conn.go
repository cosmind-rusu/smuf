package tunnel

import (
	"io"
	"net"
)

// BufConn wraps a net.Conn con un io.Reader ya existente para que
// los bytes bufferizados durante el handshake no se pierdan al pasar
// la conexión a yamux.
type BufConn struct {
	net.Conn
	r io.Reader
}

func NewBufConn(conn net.Conn, r io.Reader) *BufConn {
	return &BufConn{Conn: conn, r: r}
}

func (c *BufConn) Read(b []byte) (int, error) {
	return c.r.Read(b)
}
