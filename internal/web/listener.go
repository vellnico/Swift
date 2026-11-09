package web

import (
	"bufio"
	"io"
	"net"
	"sync"
)

// TLSOnly drops connections whose first byte is not a TLS handshake record,
// so plain-HTTP probes get silence instead of Go's "HTTP request to an HTTPS
// server" reply that fingerprints the panel (SPEC 11.1).
func TLSOnly(ln net.Listener) net.Listener { return tlsOnly{ln} }

type tlsOnly struct{ net.Listener }

func (l tlsOnly) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &firstByteConn{Conn: c, r: bufio.NewReader(c)}, nil
}

type firstByteConn struct {
	net.Conn
	r     *bufio.Reader
	once  sync.Once
	block bool
}

const tlsHandshake = 0x16

func (c *firstByteConn) Read(p []byte) (int, error) {
	c.once.Do(func() {
		b, err := c.r.Peek(1)
		if err != nil || b[0] != tlsHandshake {
			c.block = true
			c.Conn.Close()
		}
	})
	if c.block {
		return 0, io.EOF
	}
	return c.r.Read(p)
}
