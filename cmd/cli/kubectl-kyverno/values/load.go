package values

import (
	"github.com/go-git/go-billy/v5"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
)

func Load(f billy.Filesystem, filepath string) (*v1alpha1.Values, error) {
	return common.LoadYAML(f, filepath, func() *v1alpha1.Values {
		return &v1alpha1.Values{}
	})
}
