package packener

import (
	"io"
	"net"
	"os"
	"time"

	"github.com/poohvpn/po"
)

type datagramConn struct {
	localAddr    net.Addr
	remoteAddr   net.Addr
	packetCh     chan []byte
	closeOnce    po.Once
	writeFunc    func(b []byte, addr net.Addr) (n int, err error)
	readDeadline *pipeDeadline
	superDelete  func()
}

var _ net.Conn = &datagramConn{}

func (c *datagramConn) Read(b []byte) (n int, err error) {
	select {
	case p := <-c.packetCh:
		return copy(b, p), nil
	case <-c.closeOnce.Wait():
		return 0, io.EOF
	case <-c.readDeadline.wait():
		return 0, os.ErrDeadlineExceeded
	}
}

func (c *datagramConn) Write(b []byte) (n int, err error) {
	return c.writeFunc(b, c.remoteAddr)
}

func (c *datagramConn) LocalAddr() net.Addr {
	return c.localAddr
}

func (c *datagramConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (c *datagramConn) SetDeadline(t time.Time) error {
	c.readDeadline.set(t)
	return nil
}

func (c *datagramConn) SetReadDeadline(t time.Time) error {
	c.readDeadline.set(t)
	return nil
}

func (c *datagramConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func (c *datagramConn) Close() error {
	return c.closeOnce.ErrorDo(func() error {
		c.superDelete()
		return nil
	})
}
