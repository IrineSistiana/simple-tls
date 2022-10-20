package ctunnel

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"net"
	"testing"
	"time"
)

func TestOpenTunnel_IO(t *testing.T) {
	c01, c02 := net.Pipe()
	c11, c12 := net.Pipe()

	readBuf := new(bytes.Buffer)
	readDone := make(chan struct{})
	go func() {
		readBuf.ReadFrom(c12)
		close(readDone)
	}()

	data := make([]byte, 128*1024)
	rand.Read(data)

	go func() {
		if _, err := c01.Write(data); err != nil {
			t.Error(err)
		}
		c01.Close()
	}()

	err := OpenTunnel(c02, c11, TunnelOpts{IdleTimout: time.Second})
	if err != nil {
		t.Error(err)
	}
	<-readDone
	if !bytes.Equal(data, readBuf.Bytes()) {
		t.Error("data broken")
	}
}

func TestOpenTunnel_Timeout(t *testing.T) {
	_, c02 := net.Pipe()
	c11, _ := net.Pipe()

	start := time.Now()
	err := OpenTunnel(c02, c11, TunnelOpts{IdleTimout: time.Millisecond * 50})
	assert.WithinDuration(t, start, time.Now(), time.Millisecond*200, "timeout takes too long")
	if err == nil {
		t.Error("want a timeout err, but got nil")
	}
}
