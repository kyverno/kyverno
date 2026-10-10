package common

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/data"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/resource"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var policyNamespaceSelector = []byte(`{
	"apiVersion": "kyverno.io/v1",
	"kind": "ClusterPolicy",
	"metadata": {
	  "name": "enforce-pod-name"
	},
	"spec": {
	  "validationFailureAction": "audit",
	  "background": true,
	  "rules": [
		{
		  "name": "validate-name",
		  "match": {
			"resources": {
			  "kinds": [
				"Pod"
			  ],
			  "namespaceSelector": {
				"matchExpressions": [
				  {
					"key": "foo.com/managed-state",
					"operator": "In",
					"values": [
					  "managed"
					]
				  }
				]
			  }
			}
		  },
		  "validate": {
			"message": "The Pod must end with -nginx",
			"pattern": {
			  "metadata": {
				"name": "*-nginx"
			  }
			}
		  }
		}
	  ]
	}
  }
`)

func Test_GetGitBranchOrPolicyPaths(t *testing.T) {
	type TestCase struct {
		gitBranch                             string
		repoURL                               string
		policyPath                            string
		desiredBranch, actualBranch           string
		desiredPathToYAMLs, actualPathToYAMLs string
	}
	testcases := []TestCase{
		{
			gitBranch:          "main",
			repoURL:            "https://github.com/kyverno/policies",
			policyPath:         "https://github.com/kyverno/policies/openshift/team-validate-ns-name/",
			desiredBranch:      "main",
			desiredPathToYAMLs: "/openshift/team-validate-ns-name/",
		},
		{
			gitBranch:          "",
			repoURL:            "https://github.com/kyverno/policies",
			policyPath:         "https://github.com/kyverno/policies/",
			desiredBranch:      "main",
			desiredPathToYAMLs: "/",
		},
		{
			gitBranch:          "",
			repoURL:            "https://github.com/kyverno/policies",
			policyPath:         "https://github.com/kyverno/policies",
			desiredBranch:      "main",
			desiredPathToYAMLs: "/",
		},
	}

	for _, tc := range testcases {
		tc.actualBranch, tc.actualPathToYAMLs = GetGitBranchOrPolicyPaths(tc.gitBranch, tc.repoURL, tc.policyPath)
		if tc.actualBranch != tc.desiredBranch || tc.actualPathToYAMLs != tc.desiredPathToYAMLs {
			t.Errorf("Want %q got %q  OR Want %q got %q", tc.desiredBranch, tc.actualBranch, tc.desiredPathToYAMLs, tc.actualPathToYAMLs)
		}
	}
}

func Test_getSubresourceKind(t *testing.T) {
	podAPIResource := metav1.APIResource{Name: "pods", SingularName: "", Namespaced: true, Kind: "Pod"}
	podEvictionAPIResource := metav1.APIResource{Name: "pods/eviction", SingularName: "", Namespaced: true, Group: "policy", Version: "v1", Kind: "Eviction"}

	subresources := []v1alpha1.Subresource{
		{
			Subresource:    podEvictionAPIResource,
			ParentResource: podAPIResource,
		},
	}

	subresourceKind, err := getSubresourceKind("", "Pod", "eviction", subresources)
	assert.NilError(t, err)
	assert.Equal(t, subresourceKind, "Eviction")
}

func crdYAML(group, kind, plural, scope string) string {
	return `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: ` + plural + `.` + group + `
spec:
  group: ` + group + `
  names:
    kind: ` + kind + `
    plural: ` + plural + `
  scope: ` + scope + `
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
`
}

// Resources of a cluster-scoped custom kind must not be defaulted into a
// namespace once the CRD is loaded, and a CRD file must not drop the groups
// registered by the file before it (#17727).
func TestLoadCrdsFromPaths_ClusterScopedKindKeepsNoNamespace(t *testing.T) {
	data.InjectProcessor(data.NewCRDProcessor(nil))
	dir := t.TempDir()
	clusterCRD := filepath.Join(dir, "gadget.yaml")
	namespacedCRD := filepath.Join(dir, "widget.yaml")
	assert.NilError(t, os.WriteFile(clusterCRD, []byte(crdYAML("cluster.example.com", "Gadget", "gadgets", "Cluster")), 0o600))
	assert.NilError(t, os.WriteFile(namespacedCRD, []byte(crdYAML("namespaced.example.com", "Widget", "widgets", "Namespaced")), 0o600))

	assert.NilError(t, LoadCrdsFromPaths([]string{clusterCRD, namespacedCRD}))

	gadget, err := resource.YamlToUnstructured([]byte("apiVersion: cluster.example.com/v1\nkind: Gadget\nmetadata:\n  name: g\n"))
	assert.NilError(t, err)
	assert.Equal(t, gadget.GetNamespace(), "")

	widget, err := resource.YamlToUnstructured([]byte("apiVersion: namespaced.example.com/v1\nkind: Widget\nmetadata:\n  name: w\n"))
	assert.NilError(t, err)
	assert.Equal(t, widget.GetNamespace(), "default")
}
