package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/policycontext"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	pssutils "github.com/kyverno/kyverno/pkg/pss/utils"
	"github.com/kyverno/kyverno/pkg/utils/api"
	imageutils "github.com/kyverno/kyverno/pkg/utils/image"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"gotest.tools/v3/assert"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
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

func Test_PSS_AllowExistingViolations_ManagedResource(t *testing.T) {
	engineutils.ResetOwnerCaches()

	policyTs := metav1.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	enforce := kyvernov1.Enforce
	allowExisting := true
	cpol := &kyvernov1.ClusterPolicy{
		TypeMeta: metav1.TypeMeta{APIVersion: "kyverno.io/v1", Kind: "ClusterPolicy"},
		ObjectMeta: metav1.ObjectMeta{
			Name:              "pss-restricted",
			CreationTimestamp: policyTs,
		},
		Spec: kyvernov1.Spec{
			ValidationFailureAction: enforce,
			Rules: []kyvernov1.Rule{{
				Name: "restricted",
				MatchResources: kyvernov1.MatchResources{
					Any: []kyvernov1.ResourceFilter{{
						ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{"Pod"}},
					}},
				},
				Validation: &kyvernov1.Validation{
					AllowExistingViolations: &allowExisting,
					PodSecurity: &kyvernov1.PodSecurity{
						Level:   pssapi.LevelRestricted,
						Version: "latest",
					},
				},
			}},
		},
	}

	rsTemplateHash := engineutils.ComputePodTemplateHash(&corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "nginx", Image: "nginx"}}},
	}, nil)

	makePod := func(name string, ownerName string, ownerUID string) string {
		ownerRef := ""
		if ownerName != "" {
			ownerRef = fmt.Sprintf(`,
			"ownerReferences": [{
				"apiVersion": "apps/v1",
				"kind": "ReplicaSet",
				"name": "%s",
				"uid": "%s",
				"controller": true
			}]`, ownerName, ownerUID)
		}
		// Violates restricted PSS (no securityContext)
		return fmt.Sprintf(`{
			"apiVersion": "v1",
			"kind": "Pod",
			"metadata": {
				"name": "%s",
				"namespace": "default",
				"labels": {
					"app": "test",
					"pod-template-hash": "%s"
				}%s
			},
			"spec": {
				"containers": [{
					"name": "nginx",
					"image": "nginx"
				}]
			}
		}`, name, rsTemplateHash, ownerRef)
	}

	makePodWithKind := func(name string, ownerKind string, ownerName string, ownerUID string, hash string) string {
		ownerRef := ""
		if ownerName != "" {
			ownerRef = fmt.Sprintf(`,
			"ownerReferences": [{
				"apiVersion": "apps/v1",
				"kind": "%s",
				"name": "%s",
				"uid": "%s",
				"controller": true
			}]`, ownerKind, ownerName, ownerUID)
		}
		labelStr := `,
			"labels": {
				"app": "test"
			}`
		if hash != "" {
			labelStr = fmt.Sprintf(`,
			"labels": {
				"app": "test",
				"controller-revision-hash": "%s"
			}`, hash)
		}
		// Violates restricted PSS (no securityContext)
		return fmt.Sprintf(`{
			"apiVersion": "v1",
			"kind": "Pod",
			"metadata": {
				"name": "%s",
				"namespace": "default"%s%s
			},
			"spec": {
				"containers": [{
					"name": "nginx",
					"image": "nginx"
				}]
			}
		}`, name, labelStr, ownerRef)
	}

	makeObj := func(kind, namespace, name, uid string, ts metav1.Time, ownerKind, ownerName, ownerUID string) *unstructured.Unstructured {
		obj := &unstructured.Unstructured{}
		obj.SetKind(kind)
		obj.SetNamespace(namespace)
		obj.SetName(name)
		obj.SetUID(types.UID(uid))
		obj.SetCreationTimestamp(ts)
		if kind == "Job" || kind == "CronJob" {
			obj.SetAPIVersion("batch/v1")
		} else {
			obj.SetAPIVersion("apps/v1")
		}
		if ownerName != "" {
			controller := true
			obj.SetOwnerReferences([]metav1.OwnerReference{{
				APIVersion: "apps/v1",
				Kind:       ownerKind,
				Name:       ownerName,
				UID:        types.UID(ownerUID),
				Controller: &controller,
			}})
		}
		if kind == "Deployment" || kind == "ReplicaSet" {
			_ = unstructured.SetNestedMap(obj.Object, map[string]any{
				"metadata": map[string]any{
					"labels": map[string]any{"app": "test"},
				},
				"spec": map[string]any{
					"containers": []any{
						map[string]any{"name": "nginx", "image": "nginx"},
					},
				},
			}, "spec", "template")
		}
		if kind == "ControllerRevision" {
			revTemplate := map[string]any{
				"metadata": map[string]any{
					"labels": map[string]any{"app": "test"},
				},
				"spec": map[string]any{
					"containers": []any{
						map[string]any{"name": "nginx", "image": "nginx"},
					},
				},
			}
			if ownerKind == "StatefulSet" {
				_ = unstructured.SetNestedMap(obj.Object, revTemplate, "data", "spec", "template")
			} else {
				_ = unstructured.SetNestedMap(obj.Object, revTemplate, "data", "template")
			}
		}
		if kind == "ReplicaSet" {
			obj.SetLabels(map[string]string{"pod-template-hash": rsTemplateHash})
			_ = unstructured.SetNestedField(obj.Object, rsTemplateHash, "spec", "selector", "matchLabels", "pod-template-hash")
		}
		return obj
	}

	deployOldTime := metav1.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	rsOldTime := metav1.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC)
	deployNewTime := metav1.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	rsNewTime := metav1.Date(2026, time.October, 2, 0, 0, 0, 0, time.UTC)

	client := &mockValidationClient{
		resources: map[string]*unstructured.Unstructured{
			"ReplicaSet/default/rs-old":                   makeObj("ReplicaSet", "default", "rs-old", "uid-rs-old", rsOldTime, "Deployment", "deploy-old", "uid-deploy-old"),
			"Deployment/default/deploy-old":               makeObj("Deployment", "default", "deploy-old", "uid-deploy-old", deployOldTime, "", "", ""),
			"ReplicaSet/default/rs-new":                   makeObj("ReplicaSet", "default", "rs-new", "uid-rs-new", rsNewTime, "Deployment", "deploy-new", "uid-deploy-new"),
			"Deployment/default/deploy-new":               makeObj("Deployment", "default", "deploy-new", "uid-deploy-new", deployNewTime, "", "", ""),
			"ReplicaSet/default/rs-new-old-deploy":        makeObj("ReplicaSet", "default", "rs-new-old-deploy", "uid-rs-new-old-deploy", rsNewTime, "Deployment", "deploy-old", "uid-deploy-old"),
			"StatefulSet/default/sts-old":                 makeObj("StatefulSet", "default", "sts-old", "uid-sts-old", rsOldTime, "", "", ""),
			"DaemonSet/default/ds-old":                    makeObj("DaemonSet", "default", "ds-old", "uid-ds-old", rsOldTime, "", "", ""),
			"ControllerRevision/default/sts-old-prehash":  makeObj("ControllerRevision", "default", "sts-old-prehash", "uid-sts-rev-pre", rsOldTime, "StatefulSet", "sts-old", "uid-sts-old"),
			"ControllerRevision/default/sts-old-posthash": makeObj("ControllerRevision", "default", "sts-old-posthash", "uid-sts-rev-post", rsNewTime, "StatefulSet", "sts-old", "uid-sts-old"),
			"ControllerRevision/default/ds-old-prehash":   makeObj("ControllerRevision", "default", "ds-old-prehash", "uid-ds-rev-pre", rsOldTime, "DaemonSet", "ds-old", "uid-ds-old"),
			"ReplicaSet/default/rs-standalone":            makeObj("ReplicaSet", "default", "rs-standalone", "uid-rs-standalone", rsOldTime, "", "", ""),
		},
	}

	h, err := NewValidatePssHandler(client, false)
	assert.NilError(t, err)

	rule := cpol.Spec.Rules[0]
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	cfg := config.NewDefaultConfiguration(false)

	tests := []struct {
		name        string
		operation   kyvernov1.AdmissionOperation
		isAdmission bool
		podJSON     string
		oldPodJSON  string
		userInfo    *authenticationv1.UserInfo
		wantStatus  engineapi.RuleStatus
	}{
		{
			name:        "pre-policy root owner is skipped on CREATE",
			operation:   kyvernov1.Create,
			isAdmission: true,
			podJSON:     makePod("pod-old", "rs-old", "uid-rs-old"),
			userInfo: &authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:replicaset-controller",
			},
			wantStatus: engineapi.RuleStatusSkip,
		},
		{
			name:        "managed pod with empty username fails on CREATE (unauthenticated request)",
			operation:   kyvernov1.Create,
			isAdmission: true,
			podJSON:     makePod("pod-old", "rs-old", "uid-rs-old"),
			userInfo: &authenticationv1.UserInfo{
				Username: "",
			},
			wantStatus: engineapi.RuleStatusFail,
		},
		{
			name:        "managed pod with untrusted user identity fails on CREATE (forgery prevention)",
			operation:   kyvernov1.Create,
			isAdmission: true,
			podJSON:     makePod("pod-old", "rs-old", "uid-rs-old"),
			userInfo: &authenticationv1.UserInfo{
				Username: "untrusted-user",
			},
			wantStatus: engineapi.RuleStatusFail,
		},
		{
			name:        "managed pod with non-controller service account fails on CREATE (allowlist restriction)",
			operation:   kyvernov1.Create,
			isAdmission: true,
			podJSON:     makePod("pod-old", "rs-old", "uid-rs-old"),
			userInfo: &authenticationv1.UserInfo{
				Username: "system:serviceaccount:default:ci",
			},
			wantStatus: engineapi.RuleStatusFail,
		},
		{
			name:        "managed pod under new ReplicaSet of old Deployment fails on CREATE (template update)",
			operation:   kyvernov1.Create,
			isAdmission: true,
			podJSON:     makePod("pod-new-rs-old-deploy", "rs-new-old-deploy", "uid-rs-new-old-deploy"),
			userInfo: &authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:replicaset-controller",
			},
			wantStatus: engineapi.RuleStatusFail,
		},
		{
			name:        "pre-policy root owner in background scan fails on CREATE",
			operation:   kyvernov1.Create,
			isAdmission: false,
			podJSON:     makePod("pod-old", "rs-old", "uid-rs-old"),
			wantStatus:  engineapi.RuleStatusFail,
		},
		{
			name:        "post-policy root owner fails on CREATE",
			operation:   kyvernov1.Create,
			isAdmission: true,
			podJSON:     makePod("pod-new", "rs-new", "uid-rs-new"),
			userInfo: &authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:replicaset-controller",
			},
			wantStatus: engineapi.RuleStatusFail,
		},
		{
			name:        "unmanaged pod fails on CREATE",
			operation:   kyvernov1.Create,
			isAdmission: true,
			podJSON:     makePod("pod-unmanaged", "", ""),
			userInfo: &authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:replicaset-controller",
			},
			wantStatus: engineapi.RuleStatusFail,
		},
		{
			name:        "mismatched owner UID fails on CREATE",
			operation:   kyvernov1.Create,
			isAdmission: true,
			podJSON:     makePod("pod-forged", "rs-old", "forged-uid"),
			userInfo: &authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:replicaset-controller",
			},
			wantStatus: engineapi.RuleStatusFail,
		},
		{
			name:        "missing owner fails on CREATE",
			operation:   kyvernov1.Create,
			isAdmission: true,
			podJSON:     makePod("pod-missing", "rs-missing", "uid-rs-missing"),
			userInfo: &authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:replicaset-controller",
			},
			wantStatus: engineapi.RuleStatusFail,
		},
		{
			name:        "managed pod under old StatefulSet without template-revision evidence fails on CREATE",
			operation:   kyvernov1.Create,
			isAdmission: true,
			podJSON:     makePodWithKind("pod-sts-no-evidence", "StatefulSet", "sts-old", "uid-sts-old", ""),
			userInfo: &authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:statefulset-controller",
			},
			wantStatus: engineapi.RuleStatusFail,
		},
		{
			name:        "managed pod under old StatefulSet with pre-policy revision is skipped on CREATE",
			operation:   kyvernov1.Create,
			isAdmission: true,
			podJSON:     makePodWithKind("pod-sts-pre", "StatefulSet", "sts-old", "uid-sts-old", "prehash"),
			userInfo: &authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:statefulset-controller",
			},
			wantStatus: engineapi.RuleStatusSkip,
		},
		{
			name:        "managed pod under old StatefulSet with post-policy revision fails on CREATE (template update)",
			operation:   kyvernov1.Create,
			isAdmission: true,
			podJSON:     makePodWithKind("pod-sts-post", "StatefulSet", "sts-old", "uid-sts-old", "posthash"),
			userInfo: &authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:statefulset-controller",
			},
			wantStatus: engineapi.RuleStatusFail,
		},
		{
			name:        "managed pod under old DaemonSet without template-revision evidence fails on CREATE",
			operation:   kyvernov1.Create,
			isAdmission: true,
			podJSON:     makePodWithKind("pod-ds-no-evidence", "DaemonSet", "ds-old", "uid-ds-old", ""),
			userInfo: &authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:daemon-set-controller",
			},
			wantStatus: engineapi.RuleStatusFail,
		},
		{
			name:        "managed pod under old DaemonSet with pre-policy revision is skipped on CREATE",
			operation:   kyvernov1.Create,
			isAdmission: true,
			podJSON:     makePodWithKind("pod-ds-pre", "DaemonSet", "ds-old", "uid-ds-old", "prehash"),
			userInfo: &authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:daemon-set-controller",
			},
			wantStatus: engineapi.RuleStatusSkip,
		},
		{
			name:        "managed pod under standalone ReplicaSet fails on CREATE",
			operation:   kyvernov1.Create,
			isAdmission: true,
			podJSON:     makePodWithKind("pod-rs-standalone", "ReplicaSet", "rs-standalone", "uid-rs-standalone", ""),
			userInfo: &authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:replicaset-controller",
			},
			wantStatus: engineapi.RuleStatusFail,
		},
		{
			name:        "pre-existing violation skipped on UPDATE",
			operation:   kyvernov1.Update,
			isAdmission: true,
			podJSON:     makePod("pod-old", "rs-old", "uid-rs-old"),
			oldPodJSON:  makePod("pod-old", "rs-old", "uid-rs-old"),
			wantStatus:  engineapi.RuleStatusSkip,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engineutils.ResetOwnerCaches()
			resUnstructured, err := kubeutils.BytesToUnstructured([]byte(tt.podJSON))
			assert.NilError(t, err)

			pc, err := policycontext.NewPolicyContext(jp, *resUnstructured, tt.operation, nil, cfg)
			assert.NilError(t, err)
			pc = pc.WithPolicy(cpol).WithNewResource(*resUnstructured)
			if tt.isAdmission {
				pc = pc.WithAdmissionOperation(true)
			}
			if tt.userInfo != nil {
				pc = pc.WithAdmissionInfo(kyvernov2.RequestInfo{
					AdmissionUserInfo: *tt.userInfo,
				})
			}
			if tt.oldPodJSON != "" {
				oldUnstructured, err := kubeutils.BytesToUnstructured([]byte(tt.oldPodJSON))
				assert.NilError(t, err)
				pc = pc.WithOldResource(*oldUnstructured)
			}

			_, responses := h.Process(context.TODO(), logr.Discard(), pc, *resUnstructured, rule, nil, nil)
			assert.Equal(t, len(responses), 1)
			assert.Equal(t, responses[0].Status(), tt.wantStatus)
		})
	}
}
