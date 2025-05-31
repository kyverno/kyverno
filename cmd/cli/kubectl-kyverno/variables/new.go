package variables

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/deprecations"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/values"
)

func New(out io.Writer, fs billy.Filesystem, resourcePath string, path string, vals *v1alpha1.ValuesSpec, vars ...string) (*Variables, error) {
	// if we already have values, skip the file
	if vals == nil && path != "" {
		v, err := values.Load(fs, filepath.Join(resourcePath, path))
		if err != nil {
			return nil, fmt.Errorf("unable to load variable file: %s (%w)", path, err)
		}
		if deprecations.CheckValues(out, path, v) {
			return nil, fmt.Errorf("values file %s uses a deprecated schema — please migrate to the latest format", path)
		}
		vals = &v.ValuesSpec
	}
	variables := Variables{
		values:    vals,
		variables: parse(vars...),
	}
	return &variables, nil
}
