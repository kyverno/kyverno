package git

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
)

// defaultCloneTimeout is the maximum time allowed for a git clone operation.
const defaultCloneTimeout = 2 * time.Minute

// CloneFunc is a function type for cloning a git repository into a billy.Filesystem.
// It can be replaced in tests to avoid real network calls while still exercising
// the git-URL policy loading code path.
type CloneFunc func(path string, fs billy.Filesystem, branch string, auth http.BasicAuth) (*git.Repository, error)

func Clone(path string, fs billy.Filesystem, branch string, auth http.BasicAuth) (*git.Repository, error) {
	return CloneWithContext(context.Background(), path, fs, branch, auth)
}

func CloneWithContext(ctx context.Context, path string, fs billy.Filesystem, branch string, auth http.BasicAuth) (*git.Repository, error) {
	// If the context doesn't already have a deadline, apply a default timeout
	// to prevent indefinite hangs on network operations.
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultCloneTimeout)
		defer cancel()
	}
	co := &git.CloneOptions{
		URL:           path,
		ReferenceName: plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", branch)),
		Progress:      os.Stdout,
		SingleBranch:  true,
		Depth:         1,
	}
	if auth.Username != "" && auth.Password != "" {
		co.Auth = &auth
	}
	return git.CloneContext(ctx, memory.NewStorage(), fs, co)
}

func ListFiles(fs billy.Filesystem, path string, predicate func(fs.FileInfo) bool) ([]string, error) {
	path = filepath.Clean(path)
	if _, err := fs.Stat(path); err != nil {
		return nil, err
	}
	files, err := fs.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var results []string
	for _, file := range files {
		name := filepath.Join(path, file.Name())
		if file.IsDir() {
			children, err := ListFiles(fs, name, predicate)
			if err != nil {
				return nil, err
			}
			results = append(results, children...)
		} else if predicate(file) {
			results = append(results, name)
		}
	}
	return results, nil
}

func ListYamls(f billy.Filesystem, path string) ([]string, error) {
	return ListFiles(f, path, IsYaml)
}
