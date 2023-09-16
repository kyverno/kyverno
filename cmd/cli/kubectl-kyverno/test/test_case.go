package test

import (
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	testapi "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
)

type TestCase struct {
	Path string
	Fs   billy.Filesystem
	Test *testapi.Test
	Err  error
}

func (tc TestCase) Dir() string {
	return filepath.Clean(filepath.Dir(tc.Path))
}
