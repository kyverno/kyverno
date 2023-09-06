package variables

import (
	"fmt"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	valuesapi "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/values"
	sanitizederror "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/sanitizedError"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/values"
)

func New(fs billy.Filesystem, resourcePath string, path string, vals *valuesapi.Values, vars ...string) (*Variables, error) {
	// if we already have values, skip the file
	if vals == nil && path != "" {
		v, err := values.Load(fs, filepath.Join(resourcePath, path))
		if err != nil {
			return nil, sanitizederror.NewWithError("unable to read yaml", fmt.Errorf("Unable to load variable file: %s (%w)", path, err))
		}
		vals = v
	}
	variables := Variables{
		values:    vals,
		variables: parse(vars...),
	}
	return &variables, nil
}
