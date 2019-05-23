/*
 * MinIO Cloud Storage, (C) 2017 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package http

import (
	"bufio"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

// Test deadlineconn handles read timeout properly by reading two messages beyond deadline.
func TestBuffConnReadTimeout(t *testing.T) {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("unable to create listener. %v", err)
	}
	defer l.Close()
	serverAddr := l.Addr().String()

	tcpListener, ok := l.(*net.TCPListener)
	if !ok {
		t.Fatalf("failed to assert to net.TCPListener")
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		tcpConn, terr := tcpListener.AcceptTCP()
		if terr != nil {
			t.Errorf("failed to accept new connection. %v", terr)
			return
		}
		deadlineconn := newDeadlineConn(tcpConn, 1*time.Second, 1*time.Second, nil, nil)
		defer deadlineconn.Close()

		// Read a line
		var b = make([]byte, 12)
		_, terr = deadlineconn.Read(b)
		if terr != nil {
			t.Errorf("failed to read from client. %v", terr)
			return
		}
		received := string(b)
		if received != "message one\n" {
			t.Errorf(`server: expected: "message one\n", got: %v`, received)
			return
		}

		// Wait for more than read timeout to simulate processing.
		time.Sleep(3 * time.Second)

		_, terr = deadlineconn.Read(b)
		if terr != nil {
			t.Errorf("failed to read from client. %v", terr)
			return
		}
		received = string(b)
		if received != "message two\n" {
			t.Errorf(`server: expected: "message two\n", got: %v`, received)
			return
		}

		// Send a response.
		_, terr = io.WriteString(deadlineconn, "messages received\n")
		if terr != nil {
			t.Errorf("failed to write to client. %v", terr)
			return
		}
	}()

	c, err := net.Dial("tcp", serverAddr)
	if err != nil {
		t.Fatalf("unable to connect to server. %v", err)
	}
	defer c.Close()

	_, err = io.WriteString(c, "message one\n")
	if err != nil {
		t.Fatalf("failed to write to server. %v", err)
	}
	_, err = io.WriteString(c, "message two\n")
	if err != nil {
		t.Fatalf("failed to write to server. %v", err)
	}

	received, err := bufio.NewReader(c).ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read from server. %v", err)
	}
	if received != "messages received\n" {
		t.Fatalf(`client: expected: "messages received\n", got: %v`, received)
	}

	wg.Wait()
}
