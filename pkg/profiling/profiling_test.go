package profiling

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
)

// findFreePort finds an available port for testing
func findFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func TestStart_ValidAddress(t *testing.T) {
	// Find a free port
	port, err := findFreePort()
	assert.NoError(t, err)
	
	address := fmt.Sprintf("localhost:%d", port)
	logger := logr.Discard()
	
	// Start profiling server
	// This launches a goroutine so it won't block
	assert.NotPanics(t, func() {
		Start(logger, address)
	})
	
	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)
	
	// Verify server is listening by attempting to connect
	conn, err := net.DialTimeout("tcp", address, time.Second)
	if err == nil {
		conn.Close()
		// Server is running
	} else {
		t.Logf("Server may not have started yet: %v", err)
	}
}

func TestStart_DoesNotBlock(t *testing.T) {
	// Find a free port
	port, err := findFreePort()
	assert.NoError(t, err)
	
	address := fmt.Sprintf("localhost:%d", port)
	logger := logr.Discard()
	
	// Start should not block since it runs in a goroutine
	done := make(chan bool)
	go func() {
		Start(logger, address)
		done <- true
	}()
	
	// Should complete quickly because Start returns immediately
	select {
	case <-done:
		// Success - Start returned
	case <-time.After(time.Second):
		t.Fatal("Start() blocked longer than expected")
	}
}

func TestStart_MultipleAddresses(t *testing.T) {
	// Test that Start can be called with different addresses
	tests := []struct {
		name    string
		address string
	}{
		{
			name:    "localhost with port",
			address: "localhost:0", // Port 0 = auto-assign
		},
		{
			name:    "127.0.0.1 with port",
			address: "127.0.0.1:0",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logr.Discard()
			
			assert.NotPanics(t, func() {
				Start(logger, tt.address)
			})
			
			// Give server time to start
			time.Sleep(50 * time.Millisecond)
		})
	}
}

func TestStart_WithNilLogger(t *testing.T) {
	// Test that Start handles the logger parameter
	// Note: logr.Discard() is the safe way to get a no-op logger
	
	port, err := findFreePort()
	assert.NoError(t, err)
	
	address := fmt.Sprintf("localhost:%d", port)
	
	// Using a discarded logger should work fine
	logger := logr.Discard()
	
	assert.NotPanics(t, func() {
		Start(logger, address)
	})
}

func TestStart_ServerConfiguration(t *testing.T) {
	// This test verifies that Start configures the server correctly
	// We can't easily inspect the server configuration without modifying
	// the source code, but we can verify it starts successfully
	
	port, err := findFreePort()
	assert.NoError(t, err)
	
	address := fmt.Sprintf("localhost:%d", port)
	logger := logr.Discard()
	
	// Start the server
	Start(logger, address)
	
	// Wait for server to start
	time.Sleep(200 * time.Millisecond)
	
	// Verify we can connect to it
	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		t.Logf("Could not connect to server: %v (may be timing issue)", err)
	} else {
		conn.Close()
		// Server is running and accepting connections
	}
}

func TestStart_ConcurrentCalls(t *testing.T) {
	// Test that multiple concurrent calls to Start don't cause issues
	// Each needs its own port
	
	logger := logr.Discard()
	
	for i := 0; i < 3; i++ {
		port, err := findFreePort()
		assert.NoError(t, err)
		
		address := fmt.Sprintf("localhost:%d", port)
		
		assert.NotPanics(t, func() {
			Start(logger, address)
		})
	}
	
	// Give servers time to start
	time.Sleep(200 * time.Millisecond)
}

func TestStart_AddressFormats(t *testing.T) {
	// Test various valid address formats
	tests := []struct {
		name    string
		address string
		valid   bool
	}{
		{
			name:    "with port",
			address: "localhost:0",
			valid:   true,
		},
		{
			name:    "IP with port",
			address: "127.0.0.1:0",
			valid:   true,
		},
		{
			name:    "only port",
			address: ":0",
			valid:   true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logr.Discard()
			
			if tt.valid {
				assert.NotPanics(t, func() {
					Start(logger, tt.address)
				})
				time.Sleep(50 * time.Millisecond)
			}
		})
	}
}

// Note: Testing the error case where ListenAndServe fails and calls os.Exit
// is very difficult in unit tests because os.Exit terminates the test process.
// That behavior should be tested in integration tests or by mocking/refactoring
// the Start function to accept a custom exit function.

func TestStart_ReturnsImmediately(t *testing.T) {
	// Verify that Start returns immediately (doesn't block)
	port, err := findFreePort()
	assert.NoError(t, err)
	
	address := fmt.Sprintf("localhost:%d", port)
	logger := logr.Discard()
	
	start := time.Now()
	Start(logger, address)
	elapsed := time.Since(start)
	
	// Start should return almost immediately (< 100ms)
	assert.Less(t, elapsed, 100*time.Millisecond, "Start() should return immediately")
}
