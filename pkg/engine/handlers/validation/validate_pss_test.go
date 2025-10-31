package validation

import (
	"context"
	"encoding/json"
	"sort"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/policycontext"
	pssutils "github.com/kyverno/kyverno/pkg/pss/utils"
	"github.com/kyverno/kyverno/pkg/utils/api"
	imageutils "github.com/kyverno/kyverno/pkg/utils/image"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pssapi "k8s.io/pod-security-admission/api"
	"k8s.io/pod-security-admission/policy"
)

func Test_PSS_PolicyExceptions_Skip(t *testing.T) {
	// Build ClusterPolicy with PSS baseline v1.29
	cpol := &kyvernov1.ClusterPolicy{
		TypeMeta:   metav1.TypeMeta{APIVersion: "kyverno.io/v1", Kind: "ClusterPolicy"},
		ObjectMeta: metav1.ObjectMeta{Name: "psp-baseline"},
		Spec: kyvernov1.Spec{
			Rules: []kyvernov1.Rule{{
				Name: "baseline",
				MatchResources: kyvernov1.MatchResources{
					Any: []kyvernov1.ResourceFilter{{
						ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{"Pod"}},
					}},
				},
				Validation: &kyvernov1.Validation{
					PodSecurity: &kyvernov1.PodSecurity{
						Level:   pssapi.LevelBaseline,
						Version: "v1.29",
					},
				},
			}},
		},
	}

	// Pod matching the chainsaw scenario
	podJSON := `{
        "apiVersion":"v1","kind":"Pod",
        "metadata":{"name":"test-pod"},
        "spec":{
          "containers":[{
            "name":"primary","image":"alpine:latest",
            "securityContext":{
              "allowPrivilegeEscalation":false,
              "capabilities":{"drop":["ALL"]},
              "runAsNonRoot":true,
              "runAsUser":1000,
              "runAsGroup":1000,
              "seccompProfile":{"type":"RuntimeDefault"}
            }
          }],
          "initContainers":[
            {
              "name":"init1","image":"alpine:latest",
              "securityContext":{
                "allowPrivilegeEscalation":false,
                "capabilities":{"add":["NET_ADMIN","NET_RAW"],"drop":["ALL"]},
                "privileged":false,
                "readOnlyRootFilesystem":false,
                "runAsNonRoot":true,
                "runAsUser":10001,
                "runAsGroup":10001,
                "seccompProfile":{"type":"RuntimeDefault"}
              }
            },
            {
              "name":"init2","image":"busybox:latest",
              "securityContext":{
                "allowPrivilegeEscalation":false,
                "capabilities":{"add":["SYS_TIME"],"drop":["ALL"]},
                "privileged":false,
                "readOnlyRootFilesystem":true,
                "runAsNonRoot":true,
                "runAsUser":10002,
                "runAsGroup":10002,
                "seccompProfile":{"type":"RuntimeDefault"}
              }
            }
          ]
        }
      }`

	resourceUnstructured, err := kubeutils.BytesToUnstructured([]byte(podJSON))
	assert.NilError(t, err)

	// Build policy context
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	cfg := config.NewDefaultConfiguration(false)
	pc, err := policycontext.NewPolicyContext(jp, *resourceUnstructured, kyvernov1.Create, nil, cfg)
	assert.NilError(t, err)
	pc = pc.WithPolicy(cpol).WithNewResource(*resourceUnstructured)

	// Minimal admission info so MatchesException can compute images
	// Not strictly needed here

	// Exceptions matching the chainsaw test
	ex1 := &kyvernov2.PolicyException{}
	_ = json.Unmarshal([]byte(`{
      "apiVersion":"kyverno.io/v2","kind":"PolicyException",
      "metadata":{"name":"init1-exception-baseline"},
      "spec":{
        "exceptions":[{"policyName":"psp-baseline","ruleNames":["baseline"]}],
        "match":{"any":[{"resources":{"kinds":["Pod"]}}]},
        "podSecurity":[{
          "controlName":"Capabilities",
          "images":["alpine:latest"],
          "restrictedField":"spec.initContainers[*].securityContext.capabilities.add",
          "values":["NET_ADMIN","NET_RAW"]
        }]
      }
    }`), ex1)

	ex2 := &kyvernov2.PolicyException{}
	_ = json.Unmarshal([]byte(`{
      "apiVersion":"kyverno.io/v2","kind":"PolicyException",
      "metadata":{"name":"init2-exception-baseline"},
      "spec":{
        "exceptions":[{"policyName":"psp-baseline","ruleNames":["baseline"]}],
        "match":{"any":[{"resources":{"kinds":["Pod"]}}]},
        "podSecurity":[{
          "controlName":"Capabilities",
          "images":["busybox:latest"],
          "restrictedField":"spec.initContainers[*].securityContext.capabilities.add",
          "values":["SYS_TIME"]
        }]
      }
    }`), ex2)

	exceptions := []*kyvernov2.PolicyException{ex1, ex2}

	// Build rule
	rule := cpol.Spec.Rules[0]

	// Build handler with isCluster=false (avoid namespace lookup)
	h, err := NewValidatePssHandler(nil, false)
	assert.NilError(t, err)

	// Process
	logger := logr.Discard()
	_, responses := h.Process(context.TODO(), logger, pc, *resourceUnstructured, rule, nil, exceptions)

	// Expect a single RuleSkip due to exceptions
	assert.Equal(t, len(responses), 1)
	assert.Equal(t, string(responses[0].Status()), string(engineapi.RuleStatusSkip))
}

var testImages map[string]map[string]api.ImageInfo = map[string]map[string]api.ImageInfo{
	"initContainers": {
		"busybox": {
			ImageInfo: imageutils.ImageInfo{
				Registry:         "index.docker.io",
				Name:             "busybox",
				Path:             "busybox",
				Tag:              "v1.2.3",
				Reference:        "index.docker.io/busybox:v1.2.3",
				ReferenceWithTag: "index.docker.io/busybox:v1.2.3",
			},
			Pointer: "/spec/initContainers/0/image",
		},
	},
	"containers": {
		"nginx": {
			ImageInfo: imageutils.ImageInfo{
				Registry:         "docker.io",
				Name:             "nginx",
				Path:             "nginx",
				Tag:              "v13.4",
				Reference:        "docker.io/nginx:v13.4",
				ReferenceWithTag: "docker.io/nginx:v13.4",
			},
			Pointer: "/spec/containers/0/image",
		},
	},
	"ephemeralContainers": {
		"nginx2": {
			ImageInfo: imageutils.ImageInfo{
				Registry:         "docker.io",
				Name:             "nginx2",
				Path:             "test/nginx",
				Tag:              "latest",
				Reference:        "docker.io/test/nginx:latest",
				ReferenceWithTag: "docker.io/test/nginx:latest",
			},
			Pointer: "/spec/ephemeralContainers/0/image",
		},
	},
}

var testChecks []pssutils.PSSCheckResult = []pssutils.PSSCheckResult{
	{
		ID: "0",
		CheckResult: policy.CheckResult{
			Allowed:         false,
			ForbiddenReason: "---",
			ForbiddenDetail: "containers \"nginx\", \"busybox\" must set securityContext.allowPrivilegeEscalation=false",
		},
	},
	{
		ID: "1",
		CheckResult: policy.CheckResult{
			Allowed:         false,
			ForbiddenReason: "---",
			ForbiddenDetail: "containers \"nginx\", \"busybox\" must set securityContext.capabilities.drop=[\"ALL\"]",
		},
	},
	{
		ID: "2",
		CheckResult: policy.CheckResult{
			Allowed:         false,
			ForbiddenReason: "---",
			ForbiddenDetail: "pod or containers \"nginx\", \"busybox\" must set securityContext.runAsNonRoot=true",
		},
	},
	{
		ID: "3",
		CheckResult: policy.CheckResult{
			Allowed:         false,
			ForbiddenReason: "---",
			ForbiddenDetail: "pod or containers \"nginx\", \"busybox\" must set securityContext.seccompProfile.type to \"RuntimeDefault\" or \"Localhost\"",
		},
	},
	{
		ID: "4",
		CheckResult: policy.CheckResult{
			Allowed:         false,
			ForbiddenReason: "---",
			ForbiddenDetail: "pod or container \"nginx\" must set securityContext.seccompProfile.type to \"RuntimeDefault\" or \"Localhost\"",
		},
	},
	{
		ID: "5",
		CheckResult: policy.CheckResult{
			Allowed:         false,
			ForbiddenReason: "---",
			ForbiddenDetail: "container \"nginx2\" must set securityContext.allowPrivilegeEscalation=false",
		},
	},
}

func Test_addImages(t *testing.T) {
	checks := testChecks
	imageInfos := testImages
	updatedChecks := addImages(checks, imageInfos)

	assert.Equal(t, len(checks), len(updatedChecks))
	exp := []string{"docker.io/nginx:v13.4", "index.docker.io/busybox:v1.2.3"}
	got := append([]string(nil), updatedChecks[0].Images...)
	sort.Strings(exp)
	sort.Strings(got)
	assert.DeepEqual(t, got, exp)

	got = append([]string(nil), updatedChecks[1].Images...)
	sort.Strings(got)
	assert.DeepEqual(t, got, exp)

	got = append([]string(nil), updatedChecks[2].Images...)
	sort.Strings(got)
	assert.DeepEqual(t, got, exp)

	got = append([]string(nil), updatedChecks[3].Images...)
	sort.Strings(got)
	assert.DeepEqual(t, got, exp)

	assert.DeepEqual(t, updatedChecks[4].Images, []string{"docker.io/nginx:v13.4"})
	assert.DeepEqual(t, updatedChecks[5].Images, []string{"docker.io/test/nginx:latest"})

	delete(imageInfos, "ephemeralContainers")
	updatedChecks = addImages(checks, imageInfos)
	assert.DeepEqual(t, []string{"nginx2"}, updatedChecks[5].Images)
}
