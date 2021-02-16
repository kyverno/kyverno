package test

import (
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"
)

func clone(path string, fs billy.Filesystem) (*git.Repository, error) {
	return git.Clone(memory.NewStorage(), fs, &git.CloneOptions{
		URL:      path,
		Progress: os.Stdout,
	})
}

func listYAMLs(fs billy.Filesystem, path string) ([]string, error) {
	path = filepath.Clean(path)
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
