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

func getAndCompareResource(actualResources []*unstructured.Unstructured, fs billy.Filesystem, path string) (bool, string, error) {
	expectedResources, err := resource.GetResourceFromPath(fs, path)
	if err != nil {
		return false, "", fmt.Errorf("error: failed to load resource (%s)", err)
	}

	expectedResourcesMap := map[string]unstructured.Unstructured{}
	for _, expectedResource := range expectedResources {
		if expectedResource == nil {
			continue
		}
		r := *expectedResource
		resource.FixupGenerateLabels(r)
		expectedResourcesMap[expectedResource.GetNamespace()+"/"+expectedResource.GetName()] = r
	}

	for _, actualResource := range actualResources {
		if actualResource == nil {
			continue
		}
		r := *actualResource
		resource.FixupGenerateLabels(r)
		equals, err := resource.Compare(r, expectedResourcesMap[r.GetNamespace()+"/"+r.GetName()], true)
		if err != nil {
			return false, "", fmt.Errorf("error: failed to compare resources (%s)", err)
		}
		if !equals {
			log.Log.V(4).Info("Resource diff", "expected", expectedResourcesMap[r.GetNamespace()+"/"+r.GetName()], "actual", r)
			es, _ := yaml.Marshal(expectedResourcesMap[r.GetNamespace()+"/"+r.GetName()])
			as, _ := yaml.Marshal(r)
			dmp := diffmatchpatch.New()
			diffs := dmp.DiffMain(string(es), string(as), false)
			log.Log.V(4).Info("\n" + dmp.DiffPrettyText(diffs) + "\n")
			return false, dmp.DiffPrettyText(diffs), nil
		}
	}
	return true, "", nil
}
