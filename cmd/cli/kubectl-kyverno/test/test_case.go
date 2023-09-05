package test

import (
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test/api"
)

type TestCase struct {
	Path string
	Fs   billy.Filesystem
	Test *api.Test
	Err  error
}

func (tc TestCase) Dir() string {
	return filepath.Dir(tc.Path)
}
