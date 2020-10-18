//     Copyright (C) 2020, IrineSistiana
//
//     This file is part of simple-tls.
//
//     simple-tls is free software: you can redistribute it and/or modify
//     it under the terms of the GNU General Public License as published by
//     the Free Software Foundation, either version 3 of the License, or
//     (at your option) any later version.
//
//     simple-tls is distributed in the hope that it will be useful,
//     but WITHOUT ANY WARRANTY; without even the implied warranty of
//     MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//     GNU General Public License for more details.
//
//     You should have received a copy of the GNU General Public License
//     along with this program.  If not, see <https://www.gnu.org/licenses/>.

package core

import (
	"bytes"
	"io"
	"math/rand"
	"net"
	"testing"
)

func Test_paddingConn_Read_Write(t *testing.T) {
	c1, c2 := net.Pipe()
	pc1 := newPaddingConn(c1, true, true)
	pc2 := newPaddingConn(c2, true, true)

	dataSize := 512 * 1024
	data := make([]byte, dataSize)
	buf := make([]byte, dataSize)
	rand.Read(data)

	go func() {
		chunkSize := 128 * 1024
		for i := 0; i < dataSize; i += chunkSize {
			pc1.Write(data[i : i+chunkSize])
			pc1.tryWritePadding(512)
		}
		pc1.Close()
	}()

	n, err := io.ReadFull(pc2, buf)
	if n < dataSize {
		t.Fatal(err)
	}

	if !bytes.Equal(data, buf) {
		t.Fatal("data corrupted")
	}
}
