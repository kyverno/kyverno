package test

import (
	"fmt"

	"github.com/go-git/go-billy/v5"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/log"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/resource"
	"github.com/sergi/go-diff/diffmatchpatch"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func getAndCompareResource(actualResource unstructured.Unstructured, fs billy.Filesystem, path string) (bool, string, error) {
	expectedResource, err := resource.GetResourceFromPath(fs, path, actualResource.GetAPIVersion(), actualResource.GetKind(), actualResource.GetNamespace(), actualResource.GetName())
	if err != nil {
		return false, "", fmt.Errorf("error: failed to load resource (%s)", err)
	}
	resource.FixupGenerateLabels(actualResource)
	resource.FixupGenerateLabels(*expectedResource)

	equals, err := resource.Compare(actualResource, *expectedResource, true)
	if err != nil {
		return false, "", fmt.Errorf("error: failed to compare resources (%s)", err)
	}
	if !equals {
		log.Log.V(4).Info("Resource diff", "expected", expectedResource, "actual", actualResource)
		es, _ := yaml.Marshal(expectedResource)
		as, _ := yaml.Marshal(actualResource)
		dmp := diffmatchpatch.New()
		diffs := dmp.DiffMain(string(es), string(as), false)
		log.Log.V(4).Info("\n" + dmp.DiffPrettyText(diffs) + "\n")
		return false, dmp.DiffPrettyText(diffs), nil
	}
	return true, "", nil
}
