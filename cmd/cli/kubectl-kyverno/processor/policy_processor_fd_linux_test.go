//go:build linux

package processor

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"gotest.tools/v3/assert"
)

// hasOpenFD reports whether the current process holds an open descriptor whose target is the given path.
func hasOpenFD(t *testing.T, target string) bool {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	assert.NilError(t, err)
	for _, entry := range entries {
		link, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
		if err != nil {
			// the descriptor used to read the directory itself may already be gone
			continue
		}
		if link == target {
			return true
		}
	}
	return false
}

// A failed write to the mutate log must not leak the file descriptor.
func TestPrintOutput_ClosesFileOnWriteError(t *testing.T) {
	if _, err := os.Stat("/dev/full"); err != nil {
		t.Skip("/dev/full is not available")
	}
	p := &PolicyProcessor{
		MutateLogPath: "/dev/full", // every write to it fails with ENOSPC
		Out:           io.Discard,
	}
	err := p.printOutput(
		map[string]interface{}{"apiVersion": "v1", "kind": "ConfigMap"},
		engineapi.EngineResponse{},
		"resource.yaml",
		false,
	)
	assert.Assert(t, err != nil, "expected the write to /dev/full to fail")
	assert.Assert(t, !hasOpenFD(t, "/dev/full"), "file descriptor for /dev/full was leaked")
}
