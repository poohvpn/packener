package packener

import (
	"errors"
	"net"
	"sync"

	"github.com/poohvpn/po"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var QueueSize = 64

type Listener struct {
	conn      net.PacketConn
	connCh    chan *datagramConn
	sessions  sync.Map // string -> *datagramConn
	closeOnce po.Once
}

func New(conn net.PacketConn) *Listener {
	l := &Listener{
		conn:   conn,
		connCh: make(chan *datagramConn, QueueSize),
	}
	go l.run()
	return l
}

func (l *Listener) run() {
	buf := make([]byte, po.BufferSize)
	for {
		n, addr, err := l.conn.ReadFrom(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			panic(err)
		}
		index := addr.String()
		v, exist := l.sessions.Load(index)
		if !exist {
			log.Debug().Func(func(e *zerolog.Event) {
				e.Str("index", index).Str("network", addr.Network()).Msg("packener: New Session")
			})
			v, exist = l.sessions.LoadOrStore(index, &datagramConn{
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
		conn := v.(*datagramConn)
		if !exist {
			select {
			case l.connCh <- conn:
			case <-l.closeOnce.Wait():
				return
			}
		}
		select {
		case conn.packetCh <- po.Duplicate(buf[:n]):
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
			return nil, net.ErrClosed
		case conn := <-l.connCh:
			return conn, nil
		}
	}
}

func (l *Listener) Close() error {
	return l.closeOnce.ErrorDo(func() error {
		err := po.Close(l.conn)
		l.sessions.Range(func(_, v interface{}) bool {
			conn := v.(*datagramConn)
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
