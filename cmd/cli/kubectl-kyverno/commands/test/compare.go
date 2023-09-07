package test

import (
	"fmt"

	"github.com/go-git/go-billy/v5"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func getAndCompareResource(actualResource unstructured.Unstructured, fs billy.Filesystem, path string) (bool, error) {
	expectedResource, err := resource.GetResourceFromPath(fs, path)
	if err != nil {
		return false, fmt.Errorf("Error: failed to load resource (%s)", err)
	}
	resource.FixupGenerateLabels(actualResource)
	resource.FixupGenerateLabels(*expectedResource)
	equals, err := resource.Compare(actualResource, *expectedResource, true)
	if err != nil {
		return false, fmt.Errorf("Error: failed to compare resources (%s)", err)
	}
	return equals, nil
}
