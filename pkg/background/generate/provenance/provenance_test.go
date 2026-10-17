package provenance

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestEnsureKey(t *testing.T) {
	t.Parallel()
	client := fake.NewClientset()
	secrets := client.CoreV1().Secrets(config.KyvernoNamespace())
	require.NoError(t, EnsureKey(t.Context(), secrets))
	first, err := secrets.Get(t.Context(), SecretName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, corev1.SecretTypeOpaque, first.Type)
	require.NotNil(t, first.Immutable)
	assert.True(t, *first.Immutable)
	assert.Len(t, first.Data[keyField], keySize)
	assert.NotEqual(t, make([]byte, keySize), first.Data[keyField])

	// Provisioning after a replica restart must preserve the same key.
	require.NoError(t, EnsureKey(t.Context(), secrets))
	second, err := secrets.Get(t.Context(), SecretName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, first.Data, second.Data)
	assert.Equal(t, 1, actionCount(client.Actions(), "create"))
}

func TestEnsureKeyRejectsInvalidExistingSecret(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		change func(*corev1.Secret)
	}{
		{"missing key", func(secret *corev1.Secret) { secret.Data = nil }},
		{"short key", func(secret *corev1.Secret) { secret.Data[keyField] = make([]byte, keySize-1) }},
		{"long key", func(secret *corev1.Secret) { secret.Data[keyField] = make([]byte, keySize+1) }},
		{"wrong type", func(secret *corev1.Secret) { secret.Type = corev1.SecretTypeTLS }},
		{"missing immutable", func(secret *corev1.Secret) { secret.Immutable = nil }},
		{"mutable key", func(secret *corev1.Secret) { *secret.Immutable = false }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			secret := testKey()
			test.change(secret)
			client := fake.NewClientset(secret)
			err := EnsureKey(t.Context(), client.CoreV1().Secrets(config.KyvernoNamespace()))
			require.Error(t, err)
			assert.Equal(t, 0, actionCount(client.Actions(), "create"))
			assert.Equal(t, 0, actionCount(client.Actions(), "update"))
		})
	}
}

func TestEnsureKeyCreationRace(t *testing.T) {
	t.Parallel()
	for _, invalidWinner := range []bool{false, true} {
		name := "persisted winner"
		if invalidWinner {
			name = "invalid winner fails closed"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			client := fake.NewClientset()
			winner := testKey()
			if invalidWinner {
				winner.Data[keyField] = []byte("invalid")
			}
			client.PrependReactor("create", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
				require.NoError(t, client.Tracker().Add(winner))
				return true, nil, apierrors.NewAlreadyExists(schema.GroupResource{Resource: "secrets"}, SecretName)
			})
			err := EnsureKey(t.Context(), client.CoreV1().Secrets(config.KyvernoNamespace()))
			if invalidWinner {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, 2, actionCount(client.Actions(), "get"))
			assert.Equal(t, 1, actionCount(client.Actions(), "create"))
			stored, err := client.CoreV1().Secrets(config.KyvernoNamespace()).Get(t.Context(), SecretName, metav1.GetOptions{})
			require.NoError(t, err)
			assert.Equal(t, winner.Data, stored.Data)
		})
	}
}

func TestEnsureKeyAPIFailures(t *testing.T) {
	t.Parallel()
	for _, verb := range []string{"get", "create"} {
		t.Run(verb, func(t *testing.T) {
			t.Parallel()
			client := fake.NewClientset()
			client.PrependReactor(verb, "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "secrets"}, SecretName, errors.New("denied"))
			})
			err := EnsureKey(t.Context(), client.CoreV1().Secrets(config.KyvernoNamespace()))
			require.Error(t, err)
			assert.True(t, apierrors.IsForbidden(err))
		})
	}
	require.Error(t, EnsureKey(t.Context(), nil))
}

func TestProvenanceAuthenticatesEveryRoutingIdentity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		change func(*kyvernov1.Policy, *unstructured.Unstructured)
	}{
		{"target UID", func(_ *kyvernov1.Policy, obj *unstructured.Unstructured) { obj.SetUID("replacement") }},
		{"target name", func(_ *kyvernov1.Policy, obj *unstructured.Unstructured) { obj.SetName("copy") }},
		{"target namespace", func(_ *kyvernov1.Policy, obj *unstructured.Unstructured) { obj.SetNamespace("other") }},
		{"target group", func(_ *kyvernov1.Policy, obj *unstructured.Unstructured) { obj.SetAPIVersion("other.example/v1") }},
		{"target kind", func(_ *kyvernov1.Policy, obj *unstructured.Unstructured) { obj.SetKind("Secret") }},
		{"policy UID", func(policy *kyvernov1.Policy, _ *unstructured.Unstructured) { policy.UID = "replacement" }},
		{"policy name", func(policy *kyvernov1.Policy, obj *unstructured.Unstructured) {
			policy.Name = "other"
			changeLabel(obj, common.GeneratePolicyLabel, "other")
		}},
		{"policy namespace", func(policy *kyvernov1.Policy, obj *unstructured.Unstructured) {
			policy.Namespace = "other"
			changeLabel(obj, common.GeneratePolicyNamespaceLabel, "other")
		}},
	}
	for _, label := range []string{
		common.GenerateRuleLabel,
		common.GenerateTriggerUIDLabel, common.GenerateTriggerKindLabel, common.GenerateTriggerVersionLabel,
		common.GenerateTriggerGroupLabel, common.GenerateTriggerNSLabel, common.GenerateTriggerNameLabel,
		common.GenerateSourceUIDLabel, common.GenerateSourceKindLabel, common.GenerateSourceVersionLabel,
		common.GenerateSourceGroupLabel, common.GenerateSourceNSLabel, common.GenerateSourceNameLabel,
		kyverno.LabelAppManagedBy, "generate.kyverno.io/future-routing-field",
	} {
		tests = append(tests, struct {
			name   string
			change func(*kyvernov1.Policy, *unstructured.Unstructured)
		}{label, func(_ *kyvernov1.Policy, obj *unstructured.Unstructured) { changeLabel(obj, label, "changed") }})
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			store := NewStore(fake.NewClientset(testKey()))
			policy, obj := testResource()
			stamp(t, store, policy, obj)
			test.change(policy, obj)
			ok, err := store.Verify(t.Context(), policy, obj)
			require.NoError(t, err)
			assert.False(t, ok, "routing changes must invalidate a copied stamp")
		})
	}
}

func TestProvenanceAllowsAlternateServedVersion(t *testing.T) {
	t.Parallel()
	store := NewStore(fake.NewClientset(testKey()))
	policy, obj := testResource()
	obj.SetAPIVersion("example.io/v1beta1")
	obj.SetKind("Widget")
	stamp(t, store, policy, obj)
	obj.SetAPIVersion("example.io/v1")
	ok, err := store.Verify(t.Context(), policy, obj)
	require.NoError(t, err)
	assert.True(t, ok, "alternate served versions represent the same object UID")
	obj.SetAPIVersion("other.example.io/v1")
	ok, err = store.Verify(t.Context(), policy, obj)
	require.NoError(t, err)
	assert.False(t, ok, "a copied stamp must not cross API groups")
}

func TestProvenanceAllowsOrdinaryEditsAndCloneSourceMarker(t *testing.T) {
	t.Parallel()
	store := NewStore(fake.NewClientset(testKey()))
	policy, obj := testResource()
	stamp(t, store, policy, obj)
	obj.Object["data"] = map[string]interface{}{"value": "edited"}
	obj.SetResourceVersion("42")
	annotations := obj.GetAnnotations()
	annotations["example.com/user-annotation"] = "edited"
	obj.SetAnnotations(annotations)
	changeLabel(obj, "app", "user-change")
	changeLabel(obj, common.GenerateTypeCloneSourceLabel, "added-or-changed")
	ok, err := store.Verify(t.Context(), policy, obj)
	require.NoError(t, err)
	assert.True(t, ok)
	labels := obj.GetLabels()
	delete(labels, common.GenerateTypeCloneSourceLabel)
	obj.SetLabels(labels)
	ok, err = store.Verify(t.Context(), policy, obj)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestProvenanceRejectsMissingMalformedOrForgedStamp(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"", "invalid", "v2:invalid", "v1:!", "v1:", "v1:AAAA", "v1:" + strings.Repeat("A", 43)} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			store := NewStore(fake.NewClientset(testKey()))
			policy, obj := testResource()
			obj.SetAnnotations(map[string]string{Annotation: value})
			ok, err := store.Verify(t.Context(), policy, obj)
			require.NoError(t, err)
			assert.False(t, ok)
		})
	}
}

func TestProvenanceRequiresCanonicalStamp(t *testing.T) {
	t.Parallel()
	store := NewStore(fake.NewClientset(testKey()))
	policy, obj := testResource()
	stamp(t, store, policy, obj)
	valid := obj.GetAnnotations()[Annotation]
	for _, malformed := range []string{valid + "\n", valid + "=", " " + valid, valid + " "} {
		obj.SetAnnotations(map[string]string{Annotation: malformed})
		ok, err := store.Verify(t.Context(), policy, obj)
		require.NoError(t, err)
		assert.False(t, ok)
	}
}

func TestProvenanceEncodingIsStable(t *testing.T) {
	t.Parallel()
	store := NewStore(fake.NewClientset(testKey()))
	policy, obj := testResource()
	first, err := store.Sign(t.Context(), policy, obj)
	require.NoError(t, err)
	labels := obj.GetLabels()
	reordered := make(map[string]string, len(labels))
	for key, value := range labels {
		reordered[key] = value
	}
	obj.SetLabels(reordered)
	second, err := store.Sign(t.Context(), policy, obj)
	require.NoError(t, err)
	assert.Equal(t, first, second)
}

func TestProvenanceReplicaAndRestartCompatibility(t *testing.T) {
	t.Parallel()
	client := fake.NewClientset()
	require.NoError(t, EnsureKey(t.Context(), client.CoreV1().Secrets(config.KyvernoNamespace())))
	policy, obj := testResource()
	stamp(t, NewStore(client), policy, obj)
	ok, err := NewStore(client).Verify(t.Context(), policy, obj)
	require.NoError(t, err)
	assert.True(t, ok)

	// A different installation cannot authenticate even an otherwise identical
	// object and policy with copied UIDs and labels.
	otherClient := fake.NewClientset()
	require.NoError(t, EnsureKey(t.Context(), otherClient.CoreV1().Secrets(config.KyvernoNamespace())))
	ok, err = NewStore(otherClient).Verify(t.Context(), policy, obj)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestProvenanceMissingOrUnavailableKeyFailsClosed(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"missing", "forbidden", "invalid"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			policy, obj := testResource()
			stamp(t, NewStore(fake.NewClientset(testKey())), policy, obj)
			client := fake.NewClientset()
			if failure == "forbidden" {
				client.PrependReactor("get", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "secrets"}, SecretName, errors.New("denied"))
				})
			} else if failure == "invalid" {
				secret := testKey()
				secret.Data[keyField] = []byte("too short")
				require.NoError(t, client.Tracker().Add(secret))
			}
			store := NewStore(client)
			ok, err := store.Verify(t.Context(), policy, obj)
			require.Error(t, err)
			assert.False(t, ok)
			_, err = store.Sign(t.Context(), policy, obj)
			require.Error(t, err)
			assert.Equal(t, 0, actionCount(client.Actions(), "create"), "resource events must never initialize a new key")
		})
	}
}

func TestProvenanceRejectsIncompleteIdentities(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		change func(*kyvernov1.Policy, *unstructured.Unstructured)
	}{
		{"policy namespace", func(policy *kyvernov1.Policy, _ *unstructured.Unstructured) { policy.Namespace = "" }},
		{"policy UID", func(policy *kyvernov1.Policy, _ *unstructured.Unstructured) { policy.UID = "" }},
		{"policy name", func(policy *kyvernov1.Policy, _ *unstructured.Unstructured) { policy.Name = "" }},
		{"target UID", func(_ *kyvernov1.Policy, obj *unstructured.Unstructured) { obj.SetUID("") }},
		{"target name", func(_ *kyvernov1.Policy, obj *unstructured.Unstructured) { obj.SetName("") }},
		{"target kind", func(_ *kyvernov1.Policy, obj *unstructured.Unstructured) { obj.SetKind("") }},
		{"target version", func(_ *kyvernov1.Policy, obj *unstructured.Unstructured) { obj.SetAPIVersion("") }},
		{"malformed target version", func(_ *kyvernov1.Policy, obj *unstructured.Unstructured) { obj.SetAPIVersion("group/v1/invalid") }},
	}
	for _, label := range []string{common.GeneratePolicyLabel, common.GeneratePolicyNamespaceLabel, common.GenerateRuleLabel, common.GenerateTriggerUIDLabel, common.GenerateTriggerKindLabel, common.GenerateTriggerVersionLabel, kyverno.LabelAppManagedBy} {
		tests = append(tests, struct {
			name   string
			change func(*kyvernov1.Policy, *unstructured.Unstructured)
		}{label, func(_ *kyvernov1.Policy, obj *unstructured.Unstructured) { changeLabel(obj, label, "") }})
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			store := NewStore(fake.NewClientset(testKey()))
			policy, obj := testResource()
			stamp(t, store, policy, obj)
			test.change(policy, obj)
			_, err := store.Sign(t.Context(), policy, obj)
			require.Error(t, err)
			ok, err := store.Verify(t.Context(), policy, obj)
			require.NoError(t, err)
			assert.False(t, ok)
		})
	}
}

func TestProvenanceNilInputsFailClosed(t *testing.T) {
	t.Parallel()
	policy, obj := testResource()
	store := NewStore(fake.NewClientset(testKey()))
	stamp(t, store, policy, obj)
	var nilClient *fake.Clientset
	for _, nilStore := range []*Store{nil, NewStore(nil), NewStore(nilClient)} {
		_, err := nilStore.Sign(t.Context(), policy, obj)
		require.Error(t, err)
		ok, err := nilStore.Verify(t.Context(), policy, obj)
		require.Error(t, err)
		assert.False(t, ok)
	}
	var nilPolicy *kyvernov1.Policy
	for _, value := range []kyvernov1.PolicyInterface{nil, nilPolicy} {
		_, err := store.Sign(t.Context(), value, obj)
		require.Error(t, err)
		ok, err := store.Verify(t.Context(), value, obj)
		require.NoError(t, err)
		assert.False(t, ok)
	}
	_, err := store.Sign(t.Context(), policy, nil)
	require.Error(t, err)
	ok, err := store.Verify(t.Context(), policy, nil)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestProvenanceClusterScopeAndOptionalTriggerName(t *testing.T) {
	t.Parallel()
	store := NewStore(fake.NewClientset(testKey()))
	policy := &kyvernov1.ClusterPolicy{ObjectMeta: metav1.ObjectMeta{Name: "generate", UID: "cluster-policy-uid"}}
	_, obj := testResource()
	obj.SetNamespace("")
	obj.SetKind("ClusterRole")
	obj.SetAPIVersion("rbac.authorization.k8s.io/v1")
	labels := obj.GetLabels()
	labels[common.GeneratePolicyNamespaceLabel] = ""
	labels[common.GenerateTriggerNSLabel] = ""
	delete(labels, common.GenerateTriggerNameLabel)
	obj.SetLabels(labels)
	stamp(t, store, policy, obj)
	ok, err := store.Verify(t.Context(), policy, obj)
	require.NoError(t, err)
	assert.True(t, ok)

	policy.Namespace = "unexpected"
	_, err = store.Sign(t.Context(), policy, obj)
	require.Error(t, err)
}

func testKey() *corev1.Secret {
	immutable := true
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: SecretName, Namespace: config.KyvernoNamespace()},
		Type:       corev1.SecretTypeOpaque,
		Immutable:  &immutable,
		Data:       map[string][]byte{keyField: []byte("0123456789abcdef0123456789abcdef")},
	}
}

func testResource() (*kyvernov1.Policy, *unstructured.Unstructured) {
	policy := &kyvernov1.Policy{ObjectMeta: metav1.ObjectMeta{Name: "generate", Namespace: "tenant", UID: "policy-uid"}}
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name": "target", "namespace": "tenant", "uid": "target-uid",
		},
	}}
	obj.SetLabels(map[string]string{
		kyverno.LabelAppManagedBy:  kyverno.ValueKyvernoApp,
		common.GeneratePolicyLabel: policy.Name, common.GeneratePolicyNamespaceLabel: policy.Namespace,
		common.GenerateRuleLabel: "clone-config", common.GenerateTriggerNameLabel: "trigger",
		common.GenerateTriggerUIDLabel: "trigger-uid", common.GenerateTriggerKindLabel: "Namespace",
		common.GenerateTriggerVersionLabel: "v1", common.GenerateTriggerGroupLabel: "", common.GenerateTriggerNSLabel: "",
		common.GenerateSourceUIDLabel: "source-uid", common.GenerateSourceKindLabel: "ConfigMap",
		common.GenerateSourceVersionLabel: "v1", common.GenerateSourceGroupLabel: "", common.GenerateSourceNSLabel: "tenant",
		common.GenerateSourceNameLabel: "source", common.GenerateTypeCloneSourceLabel: "",
	})
	return policy, obj
}

func stamp(t *testing.T, store *Store, policy kyvernov1.PolicyInterface, obj *unstructured.Unstructured) {
	t.Helper()
	before := obj.DeepCopy()
	stamp, err := store.Sign(context.Background(), policy, obj)
	require.NoError(t, err)
	assert.Equal(t, before, obj, "Sign must not mutate its resource")
	obj.SetAnnotations(map[string]string{Annotation: stamp})
	ok, err := store.Verify(t.Context(), policy, obj)
	require.NoError(t, err)
	require.True(t, ok)
}

func changeLabel(obj *unstructured.Unstructured, key, value string) {
	labels := obj.GetLabels()
	labels[key] = value
	obj.SetLabels(labels)
}

func actionCount(actions []k8stesting.Action, verb string) int {
	count := 0
	for _, action := range actions {
		if action.GetVerb() == verb {
			count++
		}
	}
	return count
}

func TestStoreSnapshotIsBoundedToGenerationBatch(t *testing.T) {
	t.Parallel()
	client := fake.NewClientset(testKey())
	store := NewStore(client)
	batch, err := store.Snapshot(t.Context())
	require.NoError(t, err)
	policy, obj := testResource()
	for range 3 {
		stamp(t, batch, policy, obj)
		valid, err := batch.Verify(t.Context(), policy, obj)
		require.NoError(t, err)
		require.True(t, valid)
	}
	require.Equal(t, 1, actionCount(client.Actions(), "get"))
	require.NoError(t, client.CoreV1().Secrets(config.KyvernoNamespace()).Delete(t.Context(), SecretName, metav1.DeleteOptions{}))
	_, err = store.Snapshot(t.Context())
	require.Error(t, err, "a new batch must validate the key again")
}
