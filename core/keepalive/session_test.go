package keepalive

import (
	"bytes"
	"errors"
	"io"
	"math/rand"
	"net"
	"testing"
	"time"
)

func TestKeepaliveIO(t *testing.T) {
	c, s := net.Pipe()
	clientSess := NewSession(c, nil)
	serverSess := NewSession(s, &Opt{AcceptNewConnectionFromPeer: true})

	data := make([]byte, 512*1024)
	rand.Read(data)

	go func() {
		clientStream, err := clientSess.Open()
		if err != nil {
			t.Error(err)
		}
		defer clientStream.Close()
		_, err = clientStream.Write(data)
		if err != nil {
			t.Error(err)
		}
	}()

	serverStream, err := serverSess.Accept()
	if err != nil {
		t.Error(err)
	}

	b, err := io.ReadAll(serverStream)
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(data, b) {
		t.Error("data corrupted")
	}
}

func TestKeepaliveReuseIO(t *testing.T) {
	c, s := net.Pipe()
	clientSess := NewSession(c, nil)
	serverSess := NewSession(s, &Opt{AcceptNewConnectionFromPeer: true})

	data := make([]byte, 512*1024)
	rand.Read(data)

	go func() {
		for i := 0; i < 10; i++ {
			clientStream, err := clientSess.Open()
			if err != nil {
				t.Error(err)
				return
			}

			_, err = clientStream.Write(data)
			if err != nil {
				t.Error(err)
				return
			}
			clientStream.Close()
		}
	}()

	for i := 0; i < 10; i++ {
		serverStream, err := serverSess.Accept()
		if err != nil {
			t.Fatal(err)
		}

		b, err := io.ReadAll(serverStream)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, b) {
			t.Fatal("data corrupted")
		}
		serverStream.Close()
	}
}

func TestKeepaliveEcho(t *testing.T) {
	c, s := net.Pipe()
	clientSess := NewSession(c, nil)
	serverSess := NewSession(s, &Opt{AcceptNewConnectionFromPeer: true})

	data := make([]byte, 512*1024)
	rand.Read(data)

	go func() {
		for {
			serverStream, err := serverSess.Accept()
			if err != nil {
				return
			}
			if err := echo(serverStream, 16*1024); err != nil {
				return
			}
			serverStream.Close()
		}
		serverSess.Close()
	}()

	for i := 0; i < 10; i++ {
		clientStream, err := clientSess.Open()
		if err != nil {
			t.Fatal(err)
		}

		go func() {
			if _, err := clientStream.Write(data); err != nil {
				t.Error(err)
				return
			}
		}()

		buf := make([]byte, len(data))
		_, err = io.ReadFull(clientStream, buf)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, buf) {
			t.Fatal("data corrupted")
		}
		clientStream.Close()
	}
	serverSess.Close()
}

func TestStreamDDL(t *testing.T) {
	c, _ := net.Pipe()
	sess := NewSession(c, nil)
	stream, err := sess.Open()
	if err != nil {
		t.Fatal(err)
	}

	stream.SetDeadline(time.Now().Add(time.Millisecond * 10))
	for i := 0; i < 10; i++ {
		if _, err := stream.Read(make([]byte, 1)); err != ErrIOTimeout {
			t.Fatal()
		}
	}

	stream.SetDeadline(time.Time{})
	time.AfterFunc(time.Millisecond*10, func() {
		stream.Close()
	})
	if _, err := stream.Read(make([]byte, 1)); !errors.Is(err, net.ErrClosed) {
		t.Fatal()
	}
}

func echo(c net.Conn, bufSize int) error {
	_, err := io.CopyBuffer(c, c, make([]byte, bufSize))
	return err
}

func TestKeepaliveErr(t *testing.T) {
	// Test ErrOccupied
	c, _ := net.Pipe()
	sess := NewSession(c, nil)

	_, err := sess.Open()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sess.Open(); err != ErrOccupied {
		t.Fatal()
	}

	// Test io on a closed connection.
	for i := 0; i < 3; i++ {
		c, s := net.Pipe()
		sess := NewSession(c, nil)
		NewSession(s, &Opt{AcceptNewConnectionFromPeer: true}) // Run a read loop
		stream, err := sess.Open()
		if err != nil {
			t.Fatal(err)
		}
		switch i {
		case 0:
			s.Close() // Server side closed.
		case 1:
			c.Close() // Client.
		case 2:
			stream.Close() // Stream itself.
		}

		if _, err := sess.Accept(); err == nil {
			t.Fatal()
		}

		if _, err := stream.Read(make([]byte, 1)); err == nil {
			t.Fatal()
		}
		if _, err := stream.Write(make([]byte, 1)); err == nil {
			t.Fatal()
		}
	}
}

func TestKeepaliveIdleConnectionTimeout(t *testing.T) {
	c, _ := net.Pipe()
	sess := NewSession(c, &Opt{IdleConnectionTimeout: time.Millisecond})
	time.Sleep(time.Millisecond * 100)
	if !isClosedChan(sess.closeChan) || sess.closeErr != ErrIdleConnectionTimeout {
		t.Fatal()
	}

	c, _ = net.Pipe()
	sess = NewSession(c, &Opt{IdleConnectionTimeout: time.Millisecond * 50})
	stream, err := sess.Open()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond * 100)
	if isClosedChan(sess.closeChan) {
		t.Fatal()
	}
	stream.Close()
	time.Sleep(time.Millisecond * 100)
	if !isClosedChan(sess.closeChan) {
		t.Fatal()
	}
}

func Test_writeDataFrameTo(t *testing.T) {
	tests := []struct {
		name string
		n    int
	}{
		{"1", 1},
		{"4096", 4096},
		{"1024*512", 1024 * 512},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &bytes.Buffer{}
			data := make([]byte, tt.n)
			rand.Read(data)
			writeDataFrameTo(w, data)
			wantW := new(bytes.Buffer)
			remain := data
			for len(remain) > 0 {
				batch := remain
				if len(batch) > 65535 {
					batch = batch[:65535]
				}
				remain = remain[len(batch):]
				wantW.WriteByte(cmdData)
				wantW.WriteByte(byte(len(batch) >> 8))
				wantW.WriteByte(byte(len(batch)))
				wantW.Write(batch)
			}

			if !bytes.Equal(w.Bytes(), wantW.Bytes()) {
				t.Fatal()
			}
		})
	}
}

func BenchmarkKeepalive(b *testing.B) {
	c, s := net.Pipe()
	clientSess := NewSession(c, nil)
	serverSess := NewSession(s, &Opt{AcceptNewConnectionFromPeer: true})

	data := make([]byte, 16*1024)
	rand.Read(data)

	clientStream, err := clientSess.Open()
	if err != nil {
		b.Error(err)
	}
	go func() {
		defer clientStream.Close()
		for {
			_, err = clientStream.Write(data)
			if err != nil {
				b.Error(err)
				return
			}
		}
	}()

	serverStream, err := serverSess.Accept()
	if err != nil {
		b.Error(err)
	}

	buf := make([]byte, 16*1024)

	b.ResetTimer()
	b.ReportAllocs()
	rn := int64(0)
	start := time.Now()
	for i := 0; i < b.N; i++ {
		n, err := serverStream.Read(buf)
		if err != nil {
			b.Fatal(err)
		}
		rn += int64(n)
	}
	b.StopTimer()
	dur := time.Since(start).Seconds()
	b.ReportMetric(float64(rn)/(1024*1024)/dur, "M/s")
}
