package packener

import (
	"io"
	"net"
	"strings"
	"sync"

	"github.com/poohvpn/pooh"
)

var QueueSize = 64

type Listener struct {
	conn      net.PacketConn
	connCh    chan *Conn
	sessions  sync.Map // string -> *Conn
	closeOnce pooh.ErrorOnce
}

var _ net.Listener = &Listener{}

func New(conn net.PacketConn) *Listener {
	l := &Listener{
		conn:   conn,
		connCh: make(chan *Conn, QueueSize),
	}
	go l.run()
	return l
}

func (l *Listener) run() {
	buf := make([]byte, pooh.BufferSize)
	for {
		n, addr, err := l.conn.ReadFrom(buf)
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				return
			}
			panic(err)
		}
		index := addr.String()
		v, exist := l.sessions.Load(index)
		if !exist {
			v, exist = l.sessions.LoadOrStore(index, &Conn{
				localAddr:    l.Addr(),
				remoteAddr:   addr,
				packetCh:     make(chan []byte, QueueSize),
				writeFunc:    l.conn.WriteTo,
				readDeadline: makePipeDeadline(),
				superDelete: func(index string) func() {
					return func() {
						l.sessions.Delete(index)
					}
				}(index),
			})
		}
		conn := v.(*Conn)
		if !exist {
			select {
			case l.connCh <- conn:
			case <-l.closeOnce.Wait():
				return
			}
		}
		select {
		case conn.packetCh <- pooh.Duplicate(buf[:n]):
		case <-conn.closeOnce.Wait():
		case <-l.closeOnce.Wait():
			return
		}
	}
}

func (l *Listener) Accept() (net.Conn, error) {
	for {
		select {
		case <-l.closeOnce.Wait():
			return nil, io.EOF
		case conn := <-l.connCh:
			return conn, nil
		}
	}
}

func (l *Listener) Close() error {
	return l.closeOnce.Do(func() error {
		err := pooh.Close(l.conn)
		l.sessions.Range(func(_, v interface{}) bool {
			conn := v.(*Conn)
			_ = conn.Close()
			return true
		})
		return err
	})
}

func (l *Listener) Addr() net.Addr {
	return l.conn.LocalAddr()
}

func (l *Listener) closeConnCallback(index string) func() {
	return func() {
		l.sessions.Delete(index)
	}
}
