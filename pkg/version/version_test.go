package version

import (
	"runtime/debug"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
)

func TestVersionReturnsPreset(t *testing.T) {
	original := BuildVersion
	t.Cleanup(func() { BuildVersion = original })

	BuildVersion = "v1.2.3"

	got := Version()

	assert.Equal(t, "v1.2.3", got)
}

func TestVersionFallsBackToBuildInfo(t *testing.T) {
	original := BuildVersion
	t.Cleanup(func() { BuildVersion = original })

	BuildVersion = ""

	bi, ok := debug.ReadBuildInfo()
	if !ok || bi.Main.Version == "" {
		t.Skip("build info not available")
	}

	got := Version()

	assert.Equal(t, bi.Main.Version, got)
	assert.Equal(t, bi.Main.Version, BuildVersion)
}

func TestTime(t *testing.T) {
	got := Time()

	bi, ok := debug.ReadBuildInfo()
	if !ok {
		assert.Equal(t, "---", got)
		return
	}

	expected := "---"
	for _, setting := range bi.Settings {
		if setting.Key == "vcs.time" {
			expected = setting.Value
			break
		}
	}

	assert.Equal(t, expected, got)
}

func TestHash(t *testing.T) {
	got := Hash()

	bi, ok := debug.ReadBuildInfo()
	if !ok {
		assert.Equal(t, "---", got)
		return
	}

	expected := "---"
	for _, setting := range bi.Settings {
		if setting.Key == "vcs.revision" {
			expected = setting.Value
			break
		}
	}

	assert.Equal(t, expected, got)
}

func TestPrintVersionInfo(t *testing.T) {
	// Use a no-op logger to avoid actual logging
	logger := logr.Discard()

	// This should not panic
	PrintVersionInfo(logger)
}
