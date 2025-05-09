package git

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
)

func Clone(path string, fs billy.Filesystem, branch string, auth http.BasicAuth) (*git.Repository, error) {
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
	return git.Clone(memory.NewStorage(), fs, co)
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
