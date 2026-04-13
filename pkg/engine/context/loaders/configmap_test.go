package loaders

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var jp = jmespath.New(config.NewDefaultConfiguration(false))

type mockConfigmapResolver struct {
	cm  *corev1.ConfigMap
	err error
}

func (m *mockConfigmapResolver) Get(_ context.Context, namespace, name string) (*corev1.ConfigMap, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.cm, nil
}

func makeEntry(namespace string) kyvernov1.ContextEntry {
	return kyvernov1.ContextEntry{
		Name: "testcm",
		ConfigMap: &kyvernov1.ConfigMapReference{
			Name:      "sensitive-config",
			Namespace: namespace,
		},
	}
}

func makeCM(namespace string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sensitive-config",
			Namespace: namespace,
		},
		Data: map[string]string{"key": "value"},
	}
}

// Test_CrossNamespaceConfigMapAccess verifies that a namespaced policy cannot
// read ConfigMaps from a different namespace (GHSA-cvq5-hhx3-f99p).
func Test_CrossNamespaceConfigMapAccess(t *testing.T) {
	resolver := &mockConfigmapResolver{cm: makeCM("victim-ns")}
	entry := makeEntry("victim-ns")
	ctx := enginecontext.NewContext(jp)
	ldr := NewConfigMapLoader(context.TODO(), logr.Discard(), entry, resolver, ctx, "attacker-ns")
	err := ldr.LoadData()
	assert.ErrorContains(t, err, `configMap namespace "victim-ns" is different from policy namespace "attacker-ns"`)
}

func Test_CrossNamespaceConfigMapAccess_SameNamespace(t *testing.T) {
	resolver := &mockConfigmapResolver{cm: makeCM("app-ns")}
	entry := makeEntry("app-ns")
	ctx := enginecontext.NewContext(jp)
	ldr := NewConfigMapLoader(context.TODO(), logr.Discard(), entry, resolver, ctx, "app-ns")
	err := ldr.LoadData()
	assert.NilError(t, err)
}

func Test_CrossNamespaceConfigMapAccess_EmptyNamespaceDefaultsToPolicyNS(t *testing.T) {
	resolver := &mockConfigmapResolver{cm: makeCM("app-ns")}
	entry := makeEntry("")
	ctx := enginecontext.NewContext(jp)
	ldr := NewConfigMapLoader(context.TODO(), logr.Discard(), entry, resolver, ctx, "app-ns")
	err := ldr.LoadData()
	assert.NilError(t, err)
}

func Test_CrossNamespaceConfigMapAccess_ClusterPolicyUnrestricted(t *testing.T) {
	resolver := &mockConfigmapResolver{cm: makeCM("any-ns")}
	entry := makeEntry("any-ns")
	ctx := enginecontext.NewContext(jp)
	ldr := NewConfigMapLoader(context.TODO(), logr.Discard(), entry, resolver, ctx, "")
	err := ldr.LoadData()
	assert.NilError(t, err)
}

func Test_CrossNamespaceConfigMapAccess_WithVariableSubstitution(t *testing.T) {
	resolver := &mockConfigmapResolver{cm: makeCM("victim-ns")}
	entry := makeEntry("{{ targetNs }}")
	ctx := enginecontext.NewContext(jp)
	_ = ctx.AddContextEntry("targetNs", []byte(`"victim-ns"`))
	ldr := NewConfigMapLoader(context.TODO(), logr.Discard(), entry, resolver, ctx, "attacker-ns")
	err := ldr.LoadData()
	assert.ErrorContains(t, err, `configMap namespace "victim-ns" is different from policy namespace "attacker-ns"`)
}
