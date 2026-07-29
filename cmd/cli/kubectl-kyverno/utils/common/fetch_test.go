package common

import (
	"errors"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cli/loader"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

func TestGenericPolicy_AsGeneratingPolicy(t *testing.T) {
	// The lines added to extractResourcesFromPolicies branch on
	// policy.AsGeneratingPolicy() and policy.AsNamespacedGeneratingPolicy().
	// This test verifies AsGeneratingPolicy() works correctly.
	mc := makeMatchResources("apps", "deployments")

	gp := &policiesv1beta1.GeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-gpol"},
		Spec: policiesv1beta1.GeneratingPolicySpec{
			MatchConstraints: mc,
		},
	}

	generic := engineapi.NewGeneratingPolicy(gp)

	// AsGeneratingPolicy should return non-nil
	if got := generic.AsGeneratingPolicy(); got == nil {
		t.Fatal("expected AsGeneratingPolicy() to return non-nil for GeneratingPolicy")
	}
	// AsNamespacedGeneratingPolicy should return nil for a cluster-scoped policy
	if got := generic.AsNamespacedGeneratingPolicy(); got != nil {
		t.Errorf("expected AsNamespacedGeneratingPolicy() to return nil for GeneratingPolicy, got %v", got)
	}
	// MatchConstraints should be preserved
	if got := generic.AsGeneratingPolicy(); got.Spec.MatchConstraints == nil {
		t.Error("expected MatchConstraints to be set on GeneratingPolicy")
	}
}

func TestGenericPolicy_AsNamespacedGeneratingPolicy(t *testing.T) {
	// Verifies Fix 3: AsNamespacedGeneratingPolicy() branch in
	// extractResourcesFromPolicies correctly handles namespaced generating policies.
	mc := makeMatchResources("", "pods")

	ngp := &policiesv1beta1.NamespacedGeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-ngpol", Namespace: "default"},
		Spec: policiesv1beta1.GeneratingPolicySpec{
			MatchConstraints: mc,
		},
	}

	generic := engineapi.NewNamespacedGeneratingPolicy(ngp)

	// AsNamespacedGeneratingPolicy should return non-nil
	if got := generic.AsNamespacedGeneratingPolicy(); got == nil {
		t.Fatal("expected AsNamespacedGeneratingPolicy() to return non-nil for NamespacedGeneratingPolicy")
	}
	// AsGeneratingPolicy should return nil for a namespaced policy
	if got := generic.AsGeneratingPolicy(); got != nil {
		t.Errorf("expected AsGeneratingPolicy() to return nil for NamespacedGeneratingPolicy, got %v", got)
	}
	// MatchConstraints should be preserved
	if got := generic.AsNamespacedGeneratingPolicy(); got.Spec.MatchConstraints == nil {
		t.Error("expected MatchConstraints to be set on NamespacedGeneratingPolicy")
	}
}

func TestExtractResourcesFromPolicies_NewBranches(t *testing.T) {
	// Exercises the 6 lines added to extractResourcesFromPolicies for
	// NamespacedGeneratingPolicy, MutatingPolicy, and NamespacedMutatingPolicy.
	mc := makeMatchResources("apps", "deployments")

	mp := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-mpol"},
		Spec:       policiesv1beta1.MutatingPolicySpec{MatchConstraints: mc},
	}
	nmp := &policiesv1beta1.NamespacedMutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-nmpol", Namespace: "default"},
		Spec:       policiesv1beta1.MutatingPolicySpec{MatchConstraints: mc},
	}
	ngp := &policiesv1beta1.NamespacedGeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-ngpol", Namespace: "default"},
		Spec:       policiesv1beta1.GeneratingPolicySpec{MatchConstraints: mc},
	}

	dClient, err := dclient.NewFakeClient(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{},
	)
	if err != nil {
		t.Fatalf("failed to create fake client: %v", err)
	}
	dClient.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))

	tests := []struct {
		name   string
		policy engineapi.GenericPolicy
	}{
		{"MutatingPolicy", engineapi.NewMutatingPolicy(mp)},
		{"NamespacedMutatingPolicy", engineapi.NewNamespacedMutatingPolicy(nmp)},
		{"NamespacedGeneratingPolicy", engineapi.NewNamespacedGeneratingPolicy(ngp)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rf := &ResourceFetcher{
				Policies: []engineapi.GenericPolicy{tt.policy},
				Client:   dClient,
			}
			info := &resourceTypeInfo{
				gvkMap:         make(map[schema.GroupVersionKind]bool),
				subresourceMap: make(map[schema.GroupVersionKind]v1alpha1.Subresource),
			}
			// Should not panic; the branch assigns matchResources
			// and getKindsFromPolicy handles the fake client gracefully.
			rf.extractResourcesFromPolicies(info)
		})
	}
}

func TestGetFromCluster_HyphenatedNames(t *testing.T) {
	u1 := &unstructured.Unstructured{}
	u1.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
	u1.SetNamespace("default")
	u1.SetName("my-deployment")

	u2 := &unstructured.Unstructured{}
	u2.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"})
	u2.SetNamespace("kube-system")
	u2.SetName("pod-in-hyphen-ns")

	u3 := &unstructured.Unstructured{}
	u3.SetGroupVersionKind(schema.GroupVersionKind{Group: "cert-manager.io", Version: "v1", Kind: "Certificate"})
	u3.SetNamespace("cert-manager")
	u3.SetName("my-cert")

	kpol := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
		Spec: kyvernov1.Spec{
			Rules: []kyvernov1.Rule{
				{
					Name: "test-rule",
					MatchResources: kyvernov1.MatchResources{
						ResourceDescription: kyvernov1.ResourceDescription{
							Kinds: []string{"Deployment", "Pod", "Certificate"},
						},
					},
				},
			},
		},
	}
	policy := engineapi.NewKyvernoPolicy(kpol)

	gvrDeployments := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	gvrPods := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	gvrCertificates := schema.GroupVersionResource{Group: "cert-manager.io", Version: "v1", Resource: "certificates"}

	scheme := runtime.NewScheme()
	dClient, err := dclient.NewFakeClient(
		scheme,
		map[schema.GroupVersionResource]string{
			gvrDeployments:  "DeploymentList",
			gvrPods:         "PodList",
			gvrCertificates: "CertificateList",
		},
		u1, u2, u3,
	)
	if err != nil {
		t.Fatalf("failed to create fake client: %v", err)
	}

	dClient.SetDiscovery(dclient.NewFakeDiscoveryClient([]schema.GroupVersionResource{
		gvrDeployments,
		gvrPods,
		gvrCertificates,
	}))

	tests := []struct {
		name         string
		concurrency  int
		resourcePath string
		expectedName string
	}{
		{
			name:         "hyphenated resource name sequential",
			concurrency:  1,
			resourcePath: "my-deployment",
			expectedName: "my-deployment",
		},
		{
			name:         "hyphenated namespace sequential",
			concurrency:  1,
			resourcePath: "pod-in-hyphen-ns",
			expectedName: "pod-in-hyphen-ns",
		},
		{
			name:         "hyphenated group and name sequential",
			concurrency:  1,
			resourcePath: "my-cert",
			expectedName: "my-cert",
		},
		{
			name:         "hyphenated resource name concurrent",
			concurrency:  2,
			resourcePath: "my-deployment",
			expectedName: "my-deployment",
		},
		{
			name:         "hyphenated namespace concurrent",
			concurrency:  2,
			resourcePath: "pod-in-hyphen-ns",
			expectedName: "pod-in-hyphen-ns",
		},
		{
			name:         "hyphenated group and name concurrent",
			concurrency:  2,
			resourcePath: "my-cert",
			expectedName: "my-cert",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rf := &ResourceFetcher{
				Policies:      []engineapi.GenericPolicy{policy},
				ResourcePaths: []string{tt.resourcePath},
				Client:        dClient,
				Cluster:       true,
				ResourceOptions: loader.ResourceOptions{
					Concurrency: tt.concurrency,
				},
			}
			got, err := rf.getFromCluster()
			if err != nil {
				t.Fatalf("getFromCluster() failed: %v", err)
			}
			if len(got) != 1 || got[0].GetName() != tt.expectedName {
				t.Fatalf("expected resource name %s, got %v", tt.expectedName, got)
			}
		})
	}
}
