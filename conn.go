package packener

import (
	"io"
	"net"
	"os"
	"time"

	"github.com/poohvpn/pooh"
)

type Conn struct {
	localAddr    net.Addr
	remoteAddr   net.Addr
	packetCh     chan []byte
	closeOnce    pooh.ErrorOnce
	writeFunc    func(b []byte, addr net.Addr) (n int, err error)
	readDeadline *pipeDeadline
	superDelete  func()
}

var _ net.Conn = &Conn{}

func (c *Conn) Read(b []byte) (n int, err error) {
	select {
	case p := <-c.packetCh:
		return copy(b, p), nil
	case <-c.closeOnce.Wait():
		return 0, io.EOF
	case <-c.readDeadline.wait():
		return 0, os.ErrDeadlineExceeded
	}
}

func (c *Conn) Write(b []byte) (n int, err error) {
	return c.writeFunc(b, c.remoteAddr)
}

func (c *Conn) LocalAddr() net.Addr {
	return c.localAddr
}

func (c *Conn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (c *Conn) SetDeadline(t time.Time) error {
	c.readDeadline.set(t)
	return nil
}

func (c *Conn) SetReadDeadline(t time.Time) error {
	c.readDeadline.set(t)
	return nil
}

func (c *Conn) SetWriteDeadline(t time.Time) error {
	return nil
}

func (c *Conn) Close() error {
	return c.closeOnce.Do(func() error {
		c.superDelete()
		return nil
	})
}
