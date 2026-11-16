package test

import (
	"fmt"
	"reflect"

	"github.com/go-git/go-billy/v5"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/log"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/payload"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/resource"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
	"github.com/sergi/go-diff/diffmatchpatch"
	"go.yaml.in/yaml/v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func getAndCompareResource(actualResource unstructured.Unstructured, fs billy.Filesystem, path string, resourceType string, jsonMode bool) (bool, string, error) {
	var expectedResource *unstructured.Unstructured
	var err error
	if jsonMode {
		var content []byte
		content, err = common.ReadFile(fs, path)
		if err == nil {
			var value any
			value, err = payload.Parse(path, content)
			if err == nil {
				object, ok := value.(map[string]any)
				if !ok {
					return false, "", fmt.Errorf("expected JSON payload must be a top-level object")
				}
				expectedResource = &unstructured.Unstructured{Object: object}
			}
		}
	} else {
		expectedResource, err = resource.GetResourceFromPath(fs, path, actualResource.GetAPIVersion(), actualResource.GetKind(), actualResource.GetNamespace(), actualResource.GetName())
	}
	if err != nil {
		return false, "", fmt.Errorf("error: failed to load %s resource (%s)", resourceType, err)
	}
	var equals bool
	if jsonMode {
		equals = reflect.DeepEqual(actualResource.Object, expectedResource.Object)
	} else {
		resource.FixupGenerateLabels(actualResource)
		resource.FixupGenerateLabels(*expectedResource)
		equals, err = resource.Compare(actualResource, *expectedResource, true)
	}
	if err != nil {
		return false, "", fmt.Errorf("error: failed to compare %s resources (%s)", resourceType, err)
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
