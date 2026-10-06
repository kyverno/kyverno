package aggregate

import (
	"sync"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
)

// captureSink is a minimal thread-safe logr.LogSink that records Info messages.
type captureSink struct {
	mu   sync.Mutex
	msgs []string
}

func (s *captureSink) Init(logr.RuntimeInfo) {}

func (s *captureSink) Enabled(int) bool { return true }

func (s *captureSink) Info(_ int, msg string, _ ...any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.msgs = append(s.msgs, msg)
}

func (s *captureSink) Error(error, string, ...any) {}

func (s *captureSink) WithValues(...any) logr.LogSink { return s }

func (s *captureSink) WithName(string) logr.LogSink { return s }

func (s *captureSink) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.msgs)
}

func TestWarnLegacyReportSourcesOnce_NoLegacyPolicies(t *testing.T) {
	sink := &captureSink{}
	logger := logr.New(sink)
	c := &controller{}
	c.warnLegacyReportSourcesOnce(logger, 0)
	c.warnLegacyReportSourcesOnce(logger, 0)
	assert.Equal(t, 0, sink.count(), "no log should be emitted when no legacy policies are present")
}

func TestWarnLegacyReportSourcesOnce_LogsOnceOnRepeatedCalls(t *testing.T) {
	sink := &captureSink{}
	logger := logr.New(sink)
	c := &controller{}
	for i := 0; i < 5; i++ {
		c.warnLegacyReportSourcesOnce(logger, 3)
	}
	assert.Equal(t, 1, sink.count(), "legacy deprecation log should be emitted exactly once across repeated calls")
	assert.Contains(t, sink.msgs[0], "deprecated legacy kyverno.io policies present")
}

func TestWarnLegacyReportSourcesOnce_LogsOnceUnderConcurrency(t *testing.T) {
	sink := &captureSink{}
	logger := logr.New(sink)
	c := &controller{}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.warnLegacyReportSourcesOnce(logger, 2)
		}()
	}
	wg.Wait()
	assert.Equal(t, 1, sink.count(), "concurrent reconciles should emit the legacy deprecation log exactly once")
}
