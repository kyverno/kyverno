package common

import (
	"errors"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"

	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type errorFile struct {
	billy.File
}

func (errorFile) Read([]byte) (int, error) {
	return 0, errors.New("read error")
}

type errorFS struct {
	billy.Filesystem
}

func (fs errorFS) Open(filename string) (billy.File, error) {
	file, err := fs.Filesystem.Open(filename)
	if err != nil {
		return nil, err
	}
	return errorFile{File: file}, nil
}

func TestReadResourceBytes_ReadError(t *testing.T) {
	baseFS := memfs.New()
	file, err := baseFS.Create("resource.yaml")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := file.Write([]byte("apiVersion: v1\nkind: ConfigMap\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	fs := errorFS{Filesystem: baseFS}
	_, err = readResourceBytes(fs, "resource.yaml")
	if err == nil {
		t.Fatalf("expected readResourceBytes to return an error")
	}
	if errors.Is(err, errOpenResourceFile) {
		t.Fatalf("expected a read error, got open error: %v", err)
	}
}

func makeMatchResources(group, resource string) *admissionregistrationv1.MatchResources {
	return &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
			{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{group},
						APIVersions: []string{"v1"},
						Resources:   []string{resource},
					},
				},
			},
		},
	}
}

func TestGenericPolicy_AsMutatingPolicy(t *testing.T) {
	// The 4 lines added to extractResourcesFromPolicies branch on
	// policy.AsMutatingPolicy() and policy.AsNamespacedMutatingPolicy().
	// This test verifies those calls work correctly for both policy types.
	mc := makeMatchResources("apps", "deployments")

	mp := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-mpol"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: mc,
		},
	}

	gp := engineapi.NewMutatingPolicy(mp)

	// AsMutatingPolicy should return non-nil
	if got := gp.AsMutatingPolicy(); got == nil {
		t.Fatal("expected AsMutatingPolicy() to return non-nil for MutatingPolicy")
	}
	// AsNamespacedMutatingPolicy should return nil for a cluster-scoped policy
	if got := gp.AsNamespacedMutatingPolicy(); got != nil {
		t.Errorf("expected AsNamespacedMutatingPolicy() to return nil for MutatingPolicy, got %v", got)
	}
	// MatchConstraints should be preserved
	if got := gp.AsMutatingPolicy(); got.Spec.MatchConstraints == nil {
		t.Error("expected MatchConstraints to be set on MutatingPolicy")
	}
}

func TestGenericPolicy_AsNamespacedMutatingPolicy(t *testing.T) {
	mc := makeMatchResources("", "pods")

	nmp := &policiesv1beta1.NamespacedMutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-nmpol", Namespace: "default"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: mc,
		},
	}

	gp := engineapi.NewNamespacedMutatingPolicy(nmp)

	// AsNamespacedMutatingPolicy should return non-nil
	if got := gp.AsNamespacedMutatingPolicy(); got == nil {
		t.Fatal("expected AsNamespacedMutatingPolicy() to return non-nil for NamespacedMutatingPolicy")
	}
	// AsMutatingPolicy should return nil for a namespaced policy
	if got := gp.AsMutatingPolicy(); got != nil {
		t.Errorf("expected AsMutatingPolicy() to return nil for NamespacedMutatingPolicy, got %v", got)
	}
	// MatchConstraints should be preserved
	if got := gp.AsNamespacedMutatingPolicy(); got.Spec.MatchConstraints == nil {
		t.Error("expected MatchConstraints to be set on NamespacedMutatingPolicy")
	}
}
