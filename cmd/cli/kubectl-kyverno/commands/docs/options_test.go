//go:build !windows

package docs

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const docsOutputDirEnv = "KYVERNO_DOCS_PERM_TEST_DIR"

func testRootCommand() *cobra.Command {
	root := &cobra.Command{Use: "kyverno"}
	root.AddCommand(&cobra.Command{Use: "sub", Short: "A subcommand", Run: func(*cobra.Command, []string) {}})
	return root
}

// TestExecuteRestrictsOutputPermissions checks the modes requested for the docs
// output directory and the generated files. They are created in a child
// process with umask 0, so the umask of the environment running the tests
// cannot mask a permissive mode, and the umask of this shared test process is
// never changed.
func TestExecuteRestrictsOutputPermissions(t *testing.T) {
	if dir := os.Getenv(docsOutputDirEnv); dir != "" {
		syscall.Umask(0)
		if err := (options{path: dir}).execute(testRootCommand()); err != nil {
			t.Fatal(err)
		}
		return
	}

	path := filepath.Join(t.TempDir(), "nested", "docs")
	cmd := exec.Command(os.Args[0], "-test.run=^TestExecuteRestrictsOutputPermissions$") // #nosec G204 -- re-runs this test binary
	cmd.Env = append(os.Environ(), docsOutputDirEnv+"="+path)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	assert.Equal(t, os.FileMode(0o750), info.Mode().Perm())

	for _, name := range []string{"kyverno.md", "kyverno_sub.md"} {
		info, err := os.Stat(filepath.Join(path, name))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), name)
	}
}

// TestGenMarkdownTreeMatchesCobra checks that genMarkdownTree writes the same
// files and content as doc.GenMarkdownTreeCustom.
func TestGenMarkdownTreeMatchesCobra(t *testing.T) {
	want, got := t.TempDir(), t.TempDir()
	prepender := websitePrepender(true)
	require.NoError(t, doc.GenMarkdownTreeCustom(testRootCommand(), want, prepender, websiteLinkHandler))
	require.NoError(t, genMarkdownTree(testRootCommand(), got, prepender, websiteLinkHandler))

	wantEntries, err := os.ReadDir(want)
	require.NoError(t, err)
	gotEntries, err := os.ReadDir(got)
	require.NoError(t, err)
	require.Len(t, gotEntries, len(wantEntries))
	for i, e := range wantEntries {
		require.Equal(t, e.Name(), gotEntries[i].Name())
		wantContent, err := os.ReadFile(filepath.Join(want, e.Name()))
		require.NoError(t, err)
		gotContent, err := os.ReadFile(filepath.Join(got, e.Name()))
		require.NoError(t, err)
		assert.Equal(t, string(wantContent), string(gotContent), e.Name())
	}
}

// TestExecuteReturnsWriteErrors checks that errors from creating the output
// files are returned, including those from subcommand files written by the
// recursive calls.
func TestExecuteReturnsWriteErrors(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) string
		wantErr error
	}{
		{
			name: "output path has a regular-file ancestor",
			setup: func(t *testing.T) string {
				file := filepath.Join(t.TempDir(), "file")
				require.NoError(t, os.WriteFile(file, nil, 0o600))
				return filepath.Join(file, "docs")
			},
			wantErr: syscall.ENOTDIR,
		},
		{
			name: "subcommand file path is a directory",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				require.NoError(t, os.Mkdir(filepath.Join(dir, "kyverno_sub.md"), 0o750))
				return dir
			},
			wantErr: syscall.EISDIR,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := (options{path: tt.setup(t)}).execute(testRootCommand())
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}
