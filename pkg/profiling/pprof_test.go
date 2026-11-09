package profiling

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
)

type recordingSink struct {
	mu   sync.Mutex
	logs []string
	out  io.Writer
}

func (s *recordingSink) Init(info logr.RuntimeInfo) {}
func (s *recordingSink) Enabled(level int) bool     { return true }

func (s *recordingSink) Info(level int, msg string, keysAndValues ...interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs = append(s.logs, msg)
	if s.out != nil {
		fmt.Fprintf(s.out, "INFO: %s\n", msg)
	}
}

func (s *recordingSink) Error(err error, msg string, keysAndValues ...interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs = append(s.logs, msg)
	if s.out != nil {
		fmt.Fprintf(s.out, "ERROR: %s: %v\n", msg, err)
	}
}

func (s *recordingSink) WithValues(keysAndValues ...interface{}) logr.LogSink { return s }
func (s *recordingSink) WithName(name string) logr.LogSink                    { return s }

func (s *recordingSink) contains(msg string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, l := range s.logs {
		if strings.Contains(l, msg) {
			return true
		}
	}
	return false
}

func TestPprofIndexHandlerRegistered(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	rec := httptest.NewRecorder()

	http.DefaultServeMux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestPprofCmdlineHandlerRegistered(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/cmdline", nil)
	rec := httptest.NewRecorder()

	http.DefaultServeMux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestPprofProfileHandlerExists(t *testing.T) {
	// Use a canceled context so sleep(r, ...) returns immediately rather than
	// waiting for the default 30-second profiling duration.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/profile", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	http.DefaultServeMux.ServeHTTP(rec, req)

	assert.NotEqual(t, http.StatusNotFound, rec.Code)
}

func TestPprofSymbolHandlerRegistered(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/symbol", nil)
	rec := httptest.NewRecorder()

	http.DefaultServeMux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestPprofTraceHandlerExists(t *testing.T) {
	// Use a canceled context so sleep(r, ...) returns immediately rather than
	// waiting for the default 1-second trace duration.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/trace", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	http.DefaultServeMux.ServeHTTP(rec, req)

	assert.NotEqual(t, http.StatusNotFound, rec.Code)
}

func TestStartLogsAndReturnsQuickly(t *testing.T) {
	sink := &recordingSink{}
	logger := logr.New(sink)

	done := make(chan struct{})
	go func() {
		// Pass 127.0.0.1:0 directly to let the OS assign an ephemeral port,
		// avoiding TOCTOU races from opening and closing an ephemeral listener.
		Start(logger, "127.0.0.1:0")
		close(done)
	}()

	select {
	case <-done:
		// Start returned quickly without blocking the caller
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Start took longer than 100ms to return, expected non-blocking execution")
	}

	assert.True(t, sink.contains("Enable profiling, see details at"), "expected startup log message at verbosity 2")
}

func TestStartFailureExitsOnError(t *testing.T) {
	if os.Getenv("TEST_PPROF_SUBPROCESS") == "1" {
		sink := &recordingSink{out: os.Stderr}
		logger := logr.New(sink)
		addr := os.Getenv("TEST_PPROF_ADDR")
		Start(logger, addr)
		// Give time for ListenAndServe to fail and trigger os.Exit(1)
		time.Sleep(2 * time.Second)
		return
	}

	// 1. Keep a listener open so the address is guaranteed to be occupied.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	defer ln.Close()

	// 2. Launch child test process with the same test binary in a subprocess.
	cmd := exec.Command(os.Args[0], "-test.run=^TestStartFailureExitsOnError$")
	cmd.Env = append(os.Environ(), "TEST_PPROF_SUBPROCESS=1", "TEST_PPROF_ADDR="+ln.Addr().String())
	output, err := cmd.CombinedOutput()

	// 3. Assert the child reported the startup error and exited with status 1 without killing the parent.
	var exitErr *exec.ExitError
	if assert.ErrorAs(t, err, &exitErr) {
		assert.Equal(t, 1, exitErr.ExitCode())
	}
	assert.Contains(t, string(output), "failed to enable profiling")
}
