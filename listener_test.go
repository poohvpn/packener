package packener

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"

	"github.com/poohvpn/po"
)

const connAmount = 3
const payloadSize = 60000
const loopTimes = 100000

func TestListener(t *testing.T) {
	conn, err := net.ListenPacket("udp", "0.0.0.0:3333")
	if err != nil {
		t.Error(err)
	}
	listener := New(conn)
	fmt.Println("start client connections")
	var wg sync.WaitGroup
	for i := 0; i < connAmount; i++ {
		go runConn(i, &wg)
	}
	fmt.Println("start listener")
	for i := 0; i < connAmount; i++ {
		conn, err := listener.Accept()
		if err != nil {
			t.Error(err)
		}
		fmt.Println("server", i, "new conn")
		go runEcho(conn)
	}
	wg.Wait()
	err = listener.Close()
	if err != nil {
		t.Error(err)
	}
	if syncMapLen(&listener.sessions) != 0 {
		t.Error("syncMapLen(&listener.sessions)!=0")
	}
	fmt.Println("done")
}

func syncMapLen(m *sync.Map) int {
	i := 0
	m.Range(func(key, value interface{}) bool {
		i++
		return true
	})
	return i
}

func runConn(index int, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()
	conn, err := net.Dial("udp", "127.0.0.1:3333")
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	buf := make([]byte, payloadSize)
	_, err = rand.Read(buf)
	if err != nil {
		panic(err)
	}
	readBuf := make([]byte, payloadSize)
	for i := 0; i < loopTimes; i++ {
		n, err := conn.Write(buf[:payloadSize])
		if err != nil {
			panic(err)
		}
		if n != payloadSize {
			panic("read n!=payloadSize")
		}
		n, err = conn.Read(readBuf)
		if err != nil {
			panic(err)
		}
		if !bytes.Equal(buf[:n], readBuf[:n]) {
			panic("!bytes.Equal(buf[:n],readBuf[:n])")
		}
	}
	fmt.Println("client", index, "finish")

}

func runEcho(conn net.Conn) {
	buf := make([]byte, po.BufferSize)
	for i := 0; i < loopTimes; i++ {
		nr, err := conn.Read(buf)
		if err != nil {
			panic(err)
		}
		nw, err := conn.Write(buf[:nr])
		if err != nil {
			panic(err)
		}
		if nr != nw {
			panic("nr!=nw")
		}
	}
	_ = conn.Close()
	_, err := conn.Read(buf)
	if err != io.EOF {
		panic("err!=io.EOF")
	}
}
