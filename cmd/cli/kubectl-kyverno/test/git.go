package test

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
)

func clone(path string, fs billy.Filesystem, branch string) (*git.Repository, error) {
	return git.Clone(memory.NewStorage(), fs, &git.CloneOptions{
		URL:           path,
		ReferenceName: plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", branch)),
		Progress:      os.Stdout,
		SingleBranch:  true,
		Depth:         1,
	})
}

func listYAMLs(fs billy.Filesystem, path string) ([]string, error) {
	path = filepath.Clean(path)

	if _, err := fs.Stat(path); err != nil {
		return nil, err
	}

	fis, err := fs.ReadDir(path)
	if err != nil {
		return nil, err
	}
	yamls := make([]string, 0)

	for _, fi := range fis {
		name := filepath.Join(path, fi.Name())
		if fi.IsDir() {
			moreYAMLs, err := listYAMLs(fs, name)
			if err != nil {
				return nil, err
			}

			yamls = append(yamls, moreYAMLs...)
			continue
		}

		ext := filepath.Ext(name)
		if ext != ".yml" && ext != ".yaml" {
			continue
		}

		yamls = append(yamls, name)
	}
	return yamls, nil
}
