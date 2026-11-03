package engine

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	iveval "github.com/kyverno/kyverno/pkg/image/verification/evaluator"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

type mockImageContext struct {
	digestMap map[string]string
	errMap    map[string]error
}

func (m *mockImageContext) AddImages(context.Context, []string, []remote.Option, []name.Option) error {
	return nil
}

func (m *mockImageContext) Get(_ context.Context, image string, _ []remote.Option, _ []name.Option) (*imagedataloader.ImageData, error) {
	if m.errMap != nil {
		for k, err := range m.errMap {
			if strings.Contains(image, k) {
				return nil, err
			}
		}
	}
	digest := "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	if m.digestMap != nil {
		for k, d := range m.digestMap {
			if strings.Contains(image, k) {
				digest = d
				break
			}
		}
	}
	return &imagedataloader.ImageData{Digest: digest}, nil
}

func buildExtractionModeMutatingPolicy(mutateDigest bool, matchGlobs ...string) *policiesv1beta1.ImageValidatingPolicy {
	var matchRefs []policiesv1beta1.MatchImageReference
	for _, g := range matchGlobs {
		matchRefs = append(matchRefs, policiesv1beta1.MatchImageReference{Glob: g})
	}
	return &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "extraction-mutate-digest"},
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{
								admissionregistrationv1.Create,
							},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
				Mode: policieskyvernoio.EvaluationModeKubernetes,
			},
			ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{
				MutateDigest: ptr.To(mutateDigest),
				VerifyDigest: ptr.To(false),
				Required:     ptr.To(false),
			},
			AutogenConfiguration: &policiesv1beta1.ImageValidatingPolicyAutogenConfiguration{
				PodControllers: &policiesv1beta1.PodControllersGenerationConfiguration{
					Controllers: []string{"jobsets.v1alpha2.jobset.x-k8s.io"},
				},
			},
			MatchImageReferences: matchRefs,
		},
	}
}

func jobSetAdmissionRequest(obj *unstructured.Unstructured) engine.EngineRequest {
	raw, _ := obj.MarshalJSON()
	return engine.EngineRequest{
		Request: v1.AdmissionRequest{
			Operation:       v1.Create,
			Kind:            metav1.GroupVersionKind{Group: "jobset.x-k8s.io", Version: "v1alpha2", Kind: "JobSet"},
			Resource:        metav1.GroupVersionResource{Group: "jobset.x-k8s.io", Version: "v1alpha2", Resource: "jobsets"},
			RequestResource: &metav1.GroupVersionResource{Group: "jobset.x-k8s.io", Version: "v1alpha2", Resource: "jobsets"},
			Namespace:       "default",
			Name:            obj.GetName(),
			Object:          apiruntime.RawExtension{Raw: raw},
		},
		Context: libs.NewFakeContextProvider(),
	}
}

func TestHandleMutating_ExtractionMode_PinsDigest(t *testing.T) {
	policy := buildExtractionModeMutatingPolicy(true, "*")
	provider, err := NewProvider(iveval.NewCompiler(nil), []policiesv1beta1.ImageValidatingPolicyLike{policy}, nil)
	require.NoError(t, err)
	eng := NewEngine(provider, nsResolver, matching.NewMatcher(), nil, nil, config.NewDefaultConfiguration(false)).(*engineImpl)
	eng.newImageContext = func() (imagedataloader.ImageContext, error) {
		return &mockImageContext{
			digestMap: map[string]string{
				"bash:1.0": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
		}, nil
	}

	obj := jobSetWithImage("ghcr.io/repo/bash:1.0")
	req := jobSetAdmissionRequest(obj)

	resp, patches, err := eng.HandleMutating(context.Background(), req, nil)
	require.NoError(t, err)
	assert.Empty(t, resp.Policies, "successful mutation should not add error policies to response")
	require.Len(t, patches, 1)
	assert.Equal(t, "replace", patches[0].Operation)
	assert.Equal(t, "/spec/replicatedJobs/0/template/spec/template/spec/containers/0/image", patches[0].Path)
	assert.Equal(t, "ghcr.io/repo/bash:1.0@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", patches[0].Value)
}

func TestHandleMutating_ExtractionMode_MultipleTemplates(t *testing.T) {
	policy := buildExtractionModeMutatingPolicy(true, "*")
	provider, err := NewProvider(iveval.NewCompiler(nil), []policiesv1beta1.ImageValidatingPolicyLike{policy}, nil)
	require.NoError(t, err)
	eng := NewEngine(provider, nsResolver, matching.NewMatcher(), nil, nil, config.NewDefaultConfiguration(false)).(*engineImpl)
	eng.newImageContext = func() (imagedataloader.ImageContext, error) {
		return &mockImageContext{
			digestMap: map[string]string{
				"leader:v1": "sha256:1111111111111111111111111111111111111111111111111111111111111111",
				"worker:v1": "sha256:2222222222222222222222222222222222222222222222222222222222222222",
			},
		}, nil
	}

	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "jobset.x-k8s.io/v1alpha2",
		"kind":       "JobSet",
		"metadata":   map[string]any{"name": "multi-template", "namespace": "default"},
		"spec": map[string]any{
			"replicatedJobs": []any{
				map[string]any{
					"name": "leader",
					"template": map[string]any{
						"spec": map[string]any{
							"template": map[string]any{
								"spec": map[string]any{
									"containers": []any{
										map[string]any{"name": "leader", "image": "ghcr.io/repo/leader:v1"},
									},
								},
							},
						},
					},
				},
				map[string]any{
					"name": "worker",
					"template": map[string]any{
						"spec": map[string]any{
							"template": map[string]any{
								"spec": map[string]any{
									"containers": []any{
										map[string]any{"name": "worker", "image": "ghcr.io/repo/worker:v1"},
									},
								},
							},
						},
					},
				},
			},
		},
	}}
	req := jobSetAdmissionRequest(obj)

	resp, patches, err := eng.HandleMutating(context.Background(), req, nil)
	require.NoError(t, err)
	assert.Empty(t, resp.Policies)
	require.Len(t, patches, 2)
	assert.Equal(t, "/spec/replicatedJobs/0/template/spec/template/spec/containers/0/image", patches[0].Path)
	assert.Equal(t, "ghcr.io/repo/leader:v1@sha256:1111111111111111111111111111111111111111111111111111111111111111", patches[0].Value)
	assert.Equal(t, "/spec/replicatedJobs/1/template/spec/template/spec/containers/0/image", patches[1].Path)
	assert.Equal(t, "ghcr.io/repo/worker:v1@sha256:2222222222222222222222222222222222222222222222222222222222222222", patches[1].Value)
}

func TestHandleMutating_ExtractionMode_AlreadyPinnedSkipped(t *testing.T) {
	policy := buildExtractionModeMutatingPolicy(true, "*")
	provider, err := NewProvider(iveval.NewCompiler(nil), []policiesv1beta1.ImageValidatingPolicyLike{policy}, nil)
	require.NoError(t, err)
	eng := NewEngine(provider, nsResolver, matching.NewMatcher(), nil, nil, config.NewDefaultConfiguration(false)).(*engineImpl)
	eng.newImageContext = func() (imagedataloader.ImageContext, error) {
		return &mockImageContext{}, nil
	}

	obj := jobSetWithImage("ghcr.io/repo/bash:1.0@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	req := jobSetAdmissionRequest(obj)

	resp, patches, err := eng.HandleMutating(context.Background(), req, nil)
	require.NoError(t, err)
	assert.Empty(t, resp.Policies)
	assert.Empty(t, patches, "already-pinned image must not produce any patch")
}

func TestHandleMutating_ExtractionMode_MutateDigestDisabled(t *testing.T) {
	policy := buildExtractionModeMutatingPolicy(false, "*")
	provider, err := NewProvider(iveval.NewCompiler(nil), []policiesv1beta1.ImageValidatingPolicyLike{policy}, nil)
	require.NoError(t, err)
	eng := NewEngine(provider, nsResolver, matching.NewMatcher(), nil, nil, config.NewDefaultConfiguration(false)).(*engineImpl)
	eng.newImageContext = func() (imagedataloader.ImageContext, error) {
		return &mockImageContext{}, nil
	}

	obj := jobSetWithImage("ghcr.io/repo/bash:1.0")
	req := jobSetAdmissionRequest(obj)

	resp, patches, err := eng.HandleMutating(context.Background(), req, nil)
	require.NoError(t, err)
	assert.Empty(t, resp.Policies)
	assert.Empty(t, patches, "disabled mutateDigest must not produce patches")
}

func TestHandleMutating_ExtractionMode_FailedResolutionSurfacesRuleError(t *testing.T) {
	policy := buildExtractionModeMutatingPolicy(true, "*")
	provider, err := NewProvider(iveval.NewCompiler(nil), []policiesv1beta1.ImageValidatingPolicyLike{policy}, nil)
	require.NoError(t, err)
	eng := NewEngine(provider, nsResolver, matching.NewMatcher(), nil, nil, config.NewDefaultConfiguration(false)).(*engineImpl)
	eng.newImageContext = func() (imagedataloader.ImageContext, error) {
		return &mockImageContext{
			digestMap: map[string]string{
				"good:v1": "sha256:1111111111111111111111111111111111111111111111111111111111111111",
			},
			errMap: map[string]error{
				"bad:v1": errors.New("registry connection timeout"),
			},
		}, nil
	}

	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "jobset.x-k8s.io/v1alpha2",
		"kind":       "JobSet",
		"metadata":   map[string]any{"name": "partial-fail", "namespace": "default"},
		"spec": map[string]any{
			"replicatedJobs": []any{
				map[string]any{
					"name": "workers-bad",
					"template": map[string]any{
						"spec": map[string]any{
							"template": map[string]any{
								"spec": map[string]any{
									"containers": []any{
										map[string]any{"name": "c1", "image": "ghcr.io/repo/bad:v1"},
									},
								},
							},
						},
					},
				},
				map[string]any{
					"name": "workers-good",
					"template": map[string]any{
						"spec": map[string]any{
							"template": map[string]any{
								"spec": map[string]any{
									"containers": []any{
										map[string]any{"name": "c2", "image": "ghcr.io/repo/good:v1"},
									},
								},
							},
						},
					},
				},
			},
		},
	}}
	req := jobSetAdmissionRequest(obj)

	resp, patches, err := eng.HandleMutating(context.Background(), req, nil)
	require.NoError(t, err, "engine itself should not return an error return, but record RuleError")
	// Resolvable image keeps its patch
	require.Len(t, patches, 1)
	assert.Equal(t, "/spec/replicatedJobs/1/template/spec/template/spec/containers/0/image", patches[0].Path)
	assert.Equal(t, "ghcr.io/repo/good:v1@sha256:1111111111111111111111111111111111111111111111111111111111111111", patches[0].Value)

	// Failure is surfaced in policy response
	require.Len(t, resp.Policies, 1)
	rule := resp.Policies[0].Result
	assert.Equal(t, engineapi.RuleStatusError, rule.Status())
	assert.Contains(t, rule.Message(), "failed to update digest")
	assert.Contains(t, rule.Message(), "pod template at spec.replicatedJobs[0].template.spec.template")
	assert.Contains(t, rule.Message(), "registry connection timeout")
}

func TestHandleMutating_ExtractionMode_InitContainersAndContainers(t *testing.T) {
	policy := buildExtractionModeMutatingPolicy(true, "*")
	provider, err := NewProvider(iveval.NewCompiler(nil), []policiesv1beta1.ImageValidatingPolicyLike{policy}, nil)
	require.NoError(t, err)
	eng := NewEngine(provider, nsResolver, matching.NewMatcher(), nil, nil, config.NewDefaultConfiguration(false)).(*engineImpl)
	eng.newImageContext = func() (imagedataloader.ImageContext, error) {
		return &mockImageContext{
			digestMap: map[string]string{
				"init:v1": "sha256:3333333333333333333333333333333333333333333333333333333333333333",
				"app:v1":  "sha256:4444444444444444444444444444444444444444444444444444444444444444",
			},
		}, nil
	}

	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "jobset.x-k8s.io/v1alpha2",
		"kind":       "JobSet",
		"metadata":   map[string]any{"name": "init-and-containers", "namespace": "default"},
		"spec": map[string]any{
			"replicatedJobs": []any{
				map[string]any{
					"name": "worker",
					"template": map[string]any{
						"spec": map[string]any{
							"template": map[string]any{
								"spec": map[string]any{
									"initContainers": []any{
										map[string]any{"name": "init", "image": "ghcr.io/repo/init:v1"},
									},
									"containers": []any{
										map[string]any{"name": "app", "image": "ghcr.io/repo/app:v1"},
									},
								},
							},
						},
					},
				},
			},
		},
	}}
	req := jobSetAdmissionRequest(obj)

	resp, patches, err := eng.HandleMutating(context.Background(), req, nil)
	require.NoError(t, err)
	assert.Empty(t, resp.Policies)
	require.Len(t, patches, 2)
	assert.Equal(t, "replace", patches[0].Operation)
	assert.Equal(t, "/spec/replicatedJobs/0/template/spec/template/spec/initContainers/0/image", patches[0].Path)
	assert.Equal(t, "ghcr.io/repo/init:v1@sha256:3333333333333333333333333333333333333333333333333333333333333333", patches[0].Value)
	assert.Equal(t, "replace", patches[1].Operation)
	assert.Equal(t, "/spec/replicatedJobs/0/template/spec/template/spec/containers/0/image", patches[1].Path)
	assert.Equal(t, "ghcr.io/repo/app:v1@sha256:4444444444444444444444444444444444444444444444444444444444444444", patches[1].Value)
}
