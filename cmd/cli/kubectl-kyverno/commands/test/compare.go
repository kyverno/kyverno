package test

import (
	"fmt"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/resource"
	unstructuredutils "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/unstructured"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func getAndCompareResource(actualResource unstructured.Unstructured, path string, fs billy.Filesystem, policyResourcePath string) (bool, error) {
	// TODO fix the way we handle git vs non-git paths (probably at the loading phase)
	if fs == nil {
		path = filepath.Join(policyResourcePath, path)
	}
	expectedResource, err := resource.GetResourceFromPath(fs, path)
	if err != nil {
		return false, fmt.Errorf("Error: failed to load resources (%s)", err)
	}
	unstructuredutils.FixupGenerateLabels(actualResource)
	unstructuredutils.FixupGenerateLabels(*expectedResource)
	equals, err := unstructuredutils.Compare(actualResource, *expectedResource, true)
	if err != nil {
		return false, fmt.Errorf("Error: failed to compare resources (%s)", err)
	}
	return equals, nil
}
