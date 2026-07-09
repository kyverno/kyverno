//go:build integration

package mpol_test

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	mpol "github.com/kyverno/kyverno/pkg/webhooks/resource/mpol"
	"github.com/kyverno/kyverno/test/integration/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// These tests cover issue #15094: a MutatingPolicy using ApplyConfiguration to set a
// field that is an ATOMIC list/struct in the schema (initContainers[].args,
// env[].valueFrom.fieldRef, projected volume sources) was rejected with
// "invalid ApplyConfiguration: may not mutate atomic arrays, maps or structs".
// Plain Server-Side Apply (kubectl apply --server-side) applies these fine
// (ChristianCiach confirmed on the issue), so Kyverno's ApplyConfiguration should too.
//
// Each policy mirrors a real reporter on #15094. Each drives the real mpol handler +
// engine against the envtest apiserver's live pod OpenAPI schema (the same schema
// source native MutatingAdmissionPolicy uses), so it reproduces the admission-time
// behavior faithfully.

// applyConfigPolicy builds a MutatingPolicy whose single mutation is the given ApplyConfiguration
// expression, matched on annotated pods.
func applyConfigPolicy(name, expression string) *policiesv1beta1.MutatingPolicy {
	return &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			MatchConditions: []admissionregistrationv1.MatchCondition{{
				Name:       "opt-in",
				Expression: "object.metadata.annotations['inject'] == 'enabled'",
			}},
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType:          admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
				ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{Expression: expression},
			}},
		},
	}
}

func mutateAnnotatedPod(t *testing.T, policyName, podName string) (allowed bool, patched bool, patchBytes []byte) {
	t.Helper()
	h := mpol.New(testEnv.ContextProvider, engine, nil, reportutils.ReportingCfg, nil, "", &framework.MockEventGen{})
	ctx := framework.ContextWithPolicies(context.Background(), policyName)
	resp := h.MutateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest(podName, "default", []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "`+podName+`", "namespace": "default", "annotations": {"inject": "enabled"}},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)), "", time.Now())
	msg := ""
	if resp.Result != nil {
		msg = resp.Result.Message
	}
	t.Logf("[%s] Allowed=%v Patch!=nil=%v Result=%q", policyName, resp.Allowed, resp.Patch != nil, msg)
	return resp.Allowed, resp.Patch != nil, resp.Patch
}

// Korel (issue author): inject a sidecar initContainer with args (atomic []string).
func TestMutate_ApplyConfiguration_InjectsSidecarInitContainerWithArgs(t *testing.T) {
	createPolicyWithCleanup(t, applyConfigPolicy("sidecar-args", `Object{
		spec: Object.spec{
			initContainers: [
				Object.spec.initContainers{
					name: "mesh-proxy",
					image: "mesh/proxy:v1.0.0",
					args: ["proxy", "sidecar"],
					restartPolicy: "Always"
				}
			]
		}
	}`))
	waitForPolicyReady(t, 1)

	allowed, patched, patch := mutateAnnotatedPod(t, "sidecar-args", "pod-args")
	assert.True(t, allowed, "#15094: injecting initContainer args must be allowed (SSA allows it)")
	require.True(t, patched, "#15094: mutation must produce a patch, got none (atomic args rejection)")
	require.NotNil(t, findPatch(decodePatches(t, patch), "/spec/initContainers"), "sidecar initContainer must be injected")
}

// asiyani: inject an env var whose valueFrom.fieldRef is an atomic struct (ObjectFieldSelector).
func TestMutate_ApplyConfiguration_InjectsEnvWithFieldRef(t *testing.T) {
	createPolicyWithCleanup(t, applyConfigPolicy("env-fieldref", `Object{
		spec: Object.spec{
			containers: object.spec.containers.map(c, Object.spec.containers{
				name: c.name,
				env: [
					Object.spec.containers.env{
						name: "POD_NAME",
						valueFrom: Object.spec.containers.env.valueFrom{
							fieldRef: Object.spec.containers.env.valueFrom.fieldRef{
								apiVersion: "v1",
								fieldPath: "metadata.name"
							}
						}
					}
				]
			})
		}
	}`))
	waitForPolicyReady(t, 1)

	allowed, patched, patch := mutateAnnotatedPod(t, "env-fieldref", "pod-env")
	assert.True(t, allowed, "#15094: injecting env valueFrom.fieldRef (atomic struct) must be allowed")
	require.True(t, patched, "#15094: mutation must produce a patch, got none (atomic fieldRef rejection)")
	assert.Contains(t, string(patch), "POD_NAME", "the patch must actually inject the POD_NAME env var, not some unrelated change")
}

// anders-elastisys: inject a projected volume whose downwardAPI items carry an atomic fieldRef struct.
func TestMutate_ApplyConfiguration_InjectsProjectedVolumeWithFieldRef(t *testing.T) {
	createPolicyWithCleanup(t, applyConfigPolicy("projected-volume", `Object{
		spec: Object.spec{
			volumes: [
				Object.spec.volumes{
					name: "podinfo",
					projected: Object.spec.volumes.projected{
						sources: [
							Object.spec.volumes.projected.sources{
								downwardAPI: Object.spec.volumes.projected.sources.downwardAPI{
									items: [
										Object.spec.volumes.projected.sources.downwardAPI.items{
											path: "labels",
											fieldRef: Object.spec.volumes.projected.sources.downwardAPI.items.fieldRef{
												fieldPath: "metadata.labels"
											}
										}
									]
								}
							}
						]
					}
				}
			]
		}
	}`))
	waitForPolicyReady(t, 1)

	allowed, patched, patch := mutateAnnotatedPod(t, "projected-volume", "pod-vol")
	assert.True(t, allowed, "#15094: injecting a projected volume with fieldRef must be allowed")
	require.True(t, patched, "#15094: mutation must produce a patch, got none (atomic projected-source rejection)")
	require.NotNil(t, findPatch(decodePatches(t, patch), "/spec/volumes"), "projected volume must be injected")
}
