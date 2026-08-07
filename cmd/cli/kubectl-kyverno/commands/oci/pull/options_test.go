package pull

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOutputPathContained(t *testing.T) {
	dir := t.TempDir()
	names := []string{
		"policy",
		"../../tmp/pwned",
		"../../../../etc/passwd",
		"a/../../b",
		"/abs/policy",
	}
	for _, name := range names {
		pp, err := outputPath(dir, name)
		assert.NoError(t, err, "name %q", name)
		assert.True(t, strings.HasSuffix(pp, ".yaml"), "name %q -> %s", name, pp)
		rel, err := filepath.Rel(dir, pp)
		assert.NoError(t, err, "name %q -> %s", name, pp)
		escaped := rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
		assert.False(t, escaped, "policy name %q escaped output dir: %s", name, pp)
	}
}
