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

type mockConfigMapResolver struct {
	cm      *corev1.ConfigMap
	err     error
	called  bool
	gotNS   string
	gotName string
}

func (m *mockConfigMapResolver) Get(_ context.Context, namespace, name string) (*corev1.ConfigMap, error) {
	m.called = true
	m.gotNS = namespace
	m.gotName = name
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
	resolver := &mockConfigMapResolver{cm: makeCM("victim-ns")}
	entry := makeEntry("victim-ns")
	ctx := enginecontext.NewContext(jp)
	ldr := NewConfigMapLoader(context.TODO(), logr.Discard(), entry, resolver, ctx, "attacker-ns")
	err := ldr.LoadData()
	assert.ErrorContains(t, err, `configMap namespace "victim-ns" is different from policy namespace "attacker-ns"`)
	assert.Equal(t, resolver.called, false)
}

func Test_SameNamespaceConfigMapAccess(t *testing.T) {
	resolver := &mockConfigMapResolver{cm: makeCM("app-ns")}
	entry := makeEntry("app-ns")
	ctx := enginecontext.NewContext(jp)
	ldr := NewConfigMapLoader(context.TODO(), logr.Discard(), entry, resolver, ctx, "app-ns")
	err := ldr.LoadData()
	assert.NilError(t, err)
	assert.Equal(t, resolver.called, true)
	assert.Equal(t, resolver.gotNS, "app-ns")
	assert.Equal(t, resolver.gotName, "sensitive-config")
}

func Test_CrossNamespaceConfigMapAccess_EmptyNamespaceDefaultsToPolicyNS(t *testing.T) {
	resolver := &mockConfigMapResolver{cm: makeCM("app-ns")}
	entry := makeEntry("")
	ctx := enginecontext.NewContext(jp)
	ldr := NewConfigMapLoader(context.TODO(), logr.Discard(), entry, resolver, ctx, "app-ns")
	err := ldr.LoadData()
	assert.NilError(t, err)
	assert.Equal(t, resolver.called, true)
	assert.Equal(t, resolver.gotNS, "app-ns")
	assert.Equal(t, resolver.gotName, "sensitive-config")
}

func Test_CrossNamespaceConfigMapAccess_ClusterPolicyUnrestricted(t *testing.T) {
	resolver := &mockConfigMapResolver{cm: makeCM("any-ns")}
	entry := makeEntry("any-ns")
	ctx := enginecontext.NewContext(jp)
	ldr := NewConfigMapLoader(context.TODO(), logr.Discard(), entry, resolver, ctx, "")
	err := ldr.LoadData()
	assert.NilError(t, err)
	assert.Equal(t, resolver.called, true)
	assert.Equal(t, resolver.gotNS, "any-ns")
	assert.Equal(t, resolver.gotName, "sensitive-config")
}

func Test_CrossNamespaceConfigMapAccess_WithVariableSubstitution(t *testing.T) {
	resolver := &mockConfigMapResolver{cm: makeCM("victim-ns")}
	entry := makeEntry("{{ targetNs }}")
	ctx := enginecontext.NewContext(jp)
	assert.NilError(t, ctx.AddContextEntry("targetNs", []byte(`"victim-ns"`)))
	ldr := NewConfigMapLoader(context.TODO(), logr.Discard(), entry, resolver, ctx, "attacker-ns")
	err := ldr.LoadData()
	assert.ErrorContains(t, err, `configMap namespace "victim-ns" is different from policy namespace "attacker-ns"`)
	assert.Equal(t, resolver.called, false)
}

func Test_ConfigMapAccess_WithNonStringNamespaceSubstitution(t *testing.T) {
	resolver := &mockConfigMapResolver{cm: makeCM("app-ns")}
	entry := makeEntry("{{ targetNs }}")
	ctx := enginecontext.NewContext(jp)
	assert.NilError(t, ctx.AddContextEntry("targetNs", []byte(`123`)))

	ldr := NewConfigMapLoader(context.TODO(), logr.Discard(), entry, resolver, ctx, "app-ns")
	err := ldr.LoadData()

	assert.ErrorContains(t, err, "configMap.namespace")
	assert.ErrorContains(t, err, "expected string")
	assert.Equal(t, resolver.called, false)
}

func Test_ConfigMapAccess_WithNonStringNameSubstitution(t *testing.T) {
	resolver := &mockConfigMapResolver{cm: makeCM("app-ns")}
	entry := makeEntry("app-ns")
	entry.ConfigMap.Name = "{{ cmName }}"
	ctx := enginecontext.NewContext(jp)
	assert.NilError(t, ctx.AddContextEntry("cmName", []byte(`123`)))

	ldr := NewConfigMapLoader(context.TODO(), logr.Discard(), entry, resolver, ctx, "app-ns")
	err := ldr.LoadData()

	assert.ErrorContains(t, err, "configMap.name")
	assert.ErrorContains(t, err, "expected string")
	assert.Equal(t, resolver.called, false)
}
