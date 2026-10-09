package generate

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/background/generate/provenance"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// This client returns identities assigned by the API server, and applies actual
// JSON patches so UID/resourceVersion tests exercise replacement/edit races.
type provenanceStampClient struct {
	dclient.Interface
	target          *unstructured.Unstructured
	createErr       error
	collisionTarget *unstructured.Unstructured
	createCount     int
	updateCount     int
	applyCount      int
	patches         [][]byte
	beforeReturn    func(*unstructured.Unstructured)
	beforePatch     func(*unstructured.Unstructured)
}

func (c *provenanceStampClient) GetResource(_ context.Context, _, kind, _, name string, _ ...string) (*unstructured.Unstructured, error) {
	if c.target == nil || c.target.GetName() != name {
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: kind}, name)
	}
	return c.target.DeepCopy(), nil
}

func (c *provenanceStampClient) persist(obj any) *unstructured.Unstructured {
	resource := obj.(*unstructured.Unstructured).DeepCopy()
	if c.target != nil && c.target.GetName() == resource.GetName() {
		resource.SetUID(c.target.GetUID())
	} else {
		resource.SetUID(types.UID("server-" + resource.GetName()))
	}
	resource.SetResourceVersion("2")
	if c.beforeReturn != nil {
		c.beforeReturn(resource)
	}
	c.target = resource
	return resource.DeepCopy()
}

func (c *provenanceStampClient) CreateResource(_ context.Context, _, _, _ string, obj any, _ bool) (*unstructured.Unstructured, error) {
	c.createCount++
	if c.createErr != nil {
		c.target = c.collisionTarget.DeepCopy()
		return nil, c.createErr
	}
	return c.persist(obj), nil
}

func (c *provenanceStampClient) UpdateResource(_ context.Context, _, _, _ string, obj any, _ bool, _ ...string) (*unstructured.Unstructured, error) {
	c.updateCount++
	return c.persist(obj), nil
}

func (c *provenanceStampClient) ApplyResource(_ context.Context, _, _, _, _ string, obj any, _ bool, _ string, _ ...string) (*unstructured.Unstructured, error) {
	c.applyCount++
	return c.persist(obj), nil
}

func (c *provenanceStampClient) PatchResource(_ context.Context, _, _, _, _ string, patch []byte) (*unstructured.Unstructured, error) {
	c.patches = append(c.patches, append([]byte(nil), patch...))
	if c.beforePatch != nil {
		c.beforePatch(c.target)
	}
	decoded, err := jsonpatch.DecodePatch(patch)
	if err != nil {
		return nil, err
	}
	original, err := json.Marshal(c.target.Object)
	if err != nil {
		return nil, err
	}
	result, err := decoded.Apply(original)
	if err != nil {
		return nil, err
	}
	var object map[string]any
	if err := json.Unmarshal(result, &object); err != nil {
		return nil, err
	}
	c.target = &unstructured.Unstructured{Object: object}
	return c.target.DeepCopy(), nil
}

func newProvenanceStampGenerator(t *testing.T) (*generator, *provenanceStampClient) {
	t.Helper()
	client := &provenanceStampClient{Interface: dclient.NewEmptyFakeClient()}
	require.NoError(t, provenance.EnsureKey(context.Background(), client.GetKubeClient().CoreV1().Secrets(config.KyvernoNamespace())))
	pattern := kyvernov1.GeneratePattern{ResourceSpec: kyvernov1.ResourceSpec{APIVersion: "v1", Kind: "ConfigMap", Namespace: "tenant", Name: "target"}}
	pattern.SetData(map[string]any{
		"metadata": map[string]any{"uid": "requested-uid", "annotations": map[string]any{"example.com/keep": "kept"}},
		"data":     map[string]any{"key": "generated"},
	})
	policy := &kyvernov1.ClusterPolicy{ObjectMeta: metav1.ObjectMeta{Name: "generate-config", UID: "policy-uid"}}
	rule := kyvernov1.Rule{Name: "generate", Generation: &kyvernov1.Generation{GeneratePattern: pattern, Synchronize: true}}
	policy.Spec.Rules = []kyvernov1.Rule{rule}
	trigger := unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "ConfigMap",
		"metadata": map[string]any{"name": "trigger", "namespace": "tenant", "uid": "trigger-uid"},
	}}
	configuration := config.NewDefaultConfiguration(false)
	policyContext, err := engine.NewPolicyContext(jmespath.New(configuration), trigger, kyvernov1.Create, nil, configuration)
	require.NoError(t, err)
	gen := newGenerator(client, logr.Discard(), policyContext.WithPolicy(policy), policy, rule, nil, nil, trigger, pattern,
		func(context.Context, []kyvernov1.ContextEntry, enginecontext.Interface) error { return nil })
	gen.provenance = provenance.NewStore(client.GetKubeClient())
	return gen, client
}

func assertGeneratedProvenance(t *testing.T, gen *generator, client *provenanceStampClient) {
	t.Helper()
	require.NotEmpty(t, client.target.GetAnnotations()[provenance.Annotation])
	valid, err := gen.provenance.Verify(context.Background(), gen.policy, client.target)
	require.NoError(t, err)
	require.True(t, valid, "the server-returned identity must verify")
	copy := client.target.DeepCopy()
	copy.SetUID("requested-uid")
	valid, err = gen.provenance.Verify(context.Background(), gen.policy, copy)
	require.NoError(t, err)
	require.False(t, valid, "the requested identity must not be signed")
}

func TestGenerateStampsServerReturnedIdentity(t *testing.T) {
	t.Parallel()
	for _, serverSideApply := range []bool{false, true} {
		for _, existing := range []bool{false, true} {
			t.Run(fmt.Sprintf("apply=%t/existing=%t", serverSideApply, existing), func(t *testing.T) {
				t.Parallel()
				gen, client := newProvenanceStampGenerator(t)
				gen.policy.GetSpec().UseServerSideApply = serverSideApply
				if existing {
					client.target = &unstructured.Unstructured{Object: map[string]any{
						"apiVersion": "v1", "kind": "ConfigMap",
						"metadata": map[string]any{"name": "target", "namespace": "tenant", "uid": "server-target", "resourceVersion": "1"},
						"data":     map[string]any{"key": "old"},
					}}
				}
				resources, err := gen.generate()
				require.NoError(t, err)
				require.Len(t, client.patches, 1)
				assertGeneratedProvenance(t, gen, client)
				require.Equal(t, "kept", client.target.GetAnnotations()["example.com/keep"])
				require.Equal(t, "generated", client.target.Object["data"].(map[string]any)["key"])
				if !existing {
					require.Len(t, resources, 1)
					require.Equal(t, types.UID("server-target"), resources[0].UID)
				}
				if serverSideApply {
					require.Equal(t, 1, client.applyCount)
				} else if existing {
					require.Equal(t, 1, client.updateCount)
				} else {
					require.Equal(t, 1, client.createCount)
				}
			})
		}
	}
}

func TestGenerateProvenancePatchRejectsConcurrentChanges(t *testing.T) {
	t.Parallel()
	for _, change := range []string{"uid", "resourceVersion"} {
		t.Run(change, func(t *testing.T) {
			t.Parallel()
			gen, client := newProvenanceStampGenerator(t)
			client.beforePatch = func(resource *unstructured.Unstructured) {
				if change == "uid" {
					resource.SetUID("replacement-uid")
				} else {
					resource.SetResourceVersion("3")
				}
				resource.Object["data"] = map[string]any{"key": "concurrent-edit"}
			}
			_, err := gen.generate()
			require.ErrorContains(t, err, "persist generated resource provenance")
			require.Len(t, client.patches, 1)
			require.Empty(t, client.target.GetAnnotations()[provenance.Annotation])
			require.Equal(t, "concurrent-edit", client.target.Object["data"].(map[string]any)["key"])
		})
	}
}

func TestGenerateProvenanceCreateCollisionRemainsUnsigned(t *testing.T) {
	t.Parallel()
	gen, client := newProvenanceStampGenerator(t)
	client.createErr = apierrors.NewAlreadyExists(schema.GroupResource{Resource: "configmaps"}, "target")
	client.collisionTarget = &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "ConfigMap",
		"metadata": map[string]any{"name": "target", "namespace": "tenant", "uid": "collision-uid", "resourceVersion": "1"},
		"data":     map[string]any{"key": "unwritten"},
	}}
	resources, err := gen.generate()
	require.True(t, apierrors.IsAlreadyExists(err))
	require.Empty(t, resources)
	require.Equal(t, 1, client.createCount)
	require.Empty(t, client.patches)
	require.Empty(t, client.target.GetAnnotations()[provenance.Annotation])
	require.Equal(t, "unwritten", client.target.Object["data"].(map[string]any)["key"])

	// A trusted retry can authenticate this UID only after the policy really
	// updates the collision object to its intended generated content.
	client.createErr = nil
	_, err = gen.generate()
	require.NoError(t, err)
	require.Equal(t, 1, client.updateCount)
	require.Equal(t, "generated", client.target.Object["data"].(map[string]any)["key"])
	require.Equal(t, types.UID("collision-uid"), client.target.GetUID())
	assertGeneratedProvenance(t, gen, client)
}

func TestGenerateProvenanceSigningFailurePropagates(t *testing.T) {
	t.Parallel()
	gen, client := newProvenanceStampGenerator(t)
	require.NoError(t, client.GetKubeClient().CoreV1().Secrets(config.KyvernoNamespace()).Delete(context.Background(), provenance.SecretName, metav1.DeleteOptions{}))
	resources, err := gen.generate()
	require.ErrorContains(t, err, "sign generated resource provenance")
	require.Empty(t, resources)
	require.Equal(t, 1, client.createCount)
	require.Empty(t, client.patches)
	require.Empty(t, client.target.GetAnnotations()[provenance.Annotation])
}

func TestGenerateProvenanceRejectsChangedRoutingLabels(t *testing.T) {
	t.Parallel()
	gen, client := newProvenanceStampGenerator(t)
	client.beforeReturn = func(resource *unstructured.Unstructured) {
		labels := resource.GetLabels()
		labels[common.GenerateTriggerUIDLabel] = "unrelated-trigger"
		resource.SetLabels(labels)
	}
	_, err := gen.generate()
	require.ErrorContains(t, err, "routing labels changed")
	require.Empty(t, client.patches)
}

func TestGenerateControllerInitializesSigner(t *testing.T) {
	t.Parallel()
	gen, client := newProvenanceStampGenerator(t)
	controller := NewGenerateController(client, nil, nil, nil, nil, nil, nil, nil, nil, nil, logr.Discard(), nil)
	require.NotNil(t, controller.provenance)
	gen.provenance = controller.provenance
	_, err := gen.generate()
	require.NoError(t, err)
	assertGeneratedProvenance(t, gen, client)
}

func TestGenerateProvenanceCLISkipsSigning(t *testing.T) {
	t.Parallel()
	gen, client := newProvenanceStampGenerator(t)
	controller := NewGenerateControllerWithOnlyClient(client, nil)
	require.Nil(t, controller.provenance)
	gen.provenance = controller.provenance
	resources, err := gen.generate()
	require.NoError(t, err)
	require.Len(t, resources, 1)
	require.Empty(t, client.patches)
	require.Empty(t, client.target.GetAnnotations()[provenance.Annotation])
}

func TestGenerateProvenanceForeachCarriesSigner(t *testing.T) {
	t.Parallel()
	gen, client := newProvenanceStampGenerator(t)
	require.NoError(t, gen.policyContext.JSONContext().AddVariable("targets", []any{"first", "second"}))
	pattern := *gen.pattern.DeepCopy()
	pattern.Name = "{{element}}"
	gen.forEach = []kyvernov1.ForEachGeneration{{List: "targets", GeneratePattern: pattern}}
	resources, err := gen.generateForeach()
	require.NoError(t, err)
	require.Len(t, resources, 2)
	require.Len(t, client.patches, 2)
	assertGeneratedProvenance(t, gen, client)
}

func TestGenerateProvenanceStampsMatchingSynchronizedTarget(t *testing.T) {
	t.Parallel()
	gen, client := newProvenanceStampGenerator(t)
	target := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "ConfigMap",
		"metadata": map[string]any{"name": "target", "namespace": "tenant", "uid": "server-target", "resourceVersion": "1"},
		"data":     map[string]any{"key": "generated"},
	}}
	common.ManageLabels(target, gen.trigger, gen.policy, gen.rule.Name)
	client.target = target
	gen.pattern.SetData(target.Object)
	_, err := gen.generate()
	require.NoError(t, err)
	require.Zero(t, client.createCount)
	require.Zero(t, client.updateCount)
	require.Len(t, client.patches, 1)
	assertGeneratedProvenance(t, gen, client)

	// A second legitimate reconciliation must retain the stamp without writing
	// the same target and generating another annotation-update event.
	_, err = gen.generate()
	require.NoError(t, err)
	require.Zero(t, client.updateCount)
	require.Len(t, client.patches, 1)
}

// Only the context-loading method is needed by ApplyGeneratePolicy here.
type provenanceStampEngine struct {
	engineapi.Engine
}

func (provenanceStampEngine) ContextLoader(kyvernov1.PolicyInterface, kyvernov1.Rule) engineapi.EngineContextLoader {
	return func(context.Context, []kyvernov1.ContextEntry, enginecontext.Interface) error { return nil }
}

func TestGenerateControllerPropagatesMissingSigningKey(t *testing.T) {
	t.Parallel()
	gen, client := newProvenanceStampGenerator(t)
	require.NoError(t, client.GetKubeClient().CoreV1().Secrets(config.KyvernoNamespace()).Delete(context.Background(), provenance.SecretName, metav1.DeleteOptions{}))
	controller := NewGenerateController(client, nil, nil, provenanceStampEngine{}, nil, nil, nil, nil, nil, nil, logr.Discard(), nil)
	_, err := controller.ApplyGeneratePolicy(logr.Discard(), gen.policyContext.(*engine.PolicyContext), []string{gen.rule.Name})
	require.ErrorContains(t, err, "sign generated resource provenance")
	require.ErrorIs(t, err, errGeneratedProvenance)
	require.Empty(t, client.patches)
}
