package evaluator

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type recordingSecretLister struct {
	corev1listers.SecretLister
	references []string
}

type recordingSecretNamespaceLister struct {
	corev1listers.SecretNamespaceLister
	parent    *recordingSecretLister
	namespace string
}

func (l *recordingSecretLister) Secrets(namespace string) corev1listers.SecretNamespaceLister {
	return &recordingSecretNamespaceLister{parent: l, namespace: namespace}
}

func (l *recordingSecretNamespaceLister) Get(name string) (*corev1.Secret, error) {
	l.parent.references = append(l.parent.references, l.namespace+"/"+name)
	return nil, errors.New("stop at secret lookup")
}

type forbiddenRegistryTransport struct{ calls int }

func (r *forbiddenRegistryTransport) RoundTrip(*http.Request) (*http.Response, error) {
	r.calls++
	return nil, errors.New("registry access is forbidden in this test")
}

func TestCompileImagePolicyCredentialSecretScope(t *testing.T) {
	libs.GetLibsCtx()
	t.Parallel()
	for _, tc := range []struct {
		name, reference, want string
		cluster, denied       bool
	}{
		{name: "bare name", reference: "registry", want: "team-a/registry"},
		{name: "same namespace", reference: "team-a/registry", want: "team-a/registry"},
		{name: "foreign namespace", reference: "team-b/registry", denied: true},
		{name: "installation namespace", reference: "kyverno/registry", denied: true},
		{name: "malformed reference", reference: "team-a/registry/extra", denied: true},
		{name: "cluster bare name", reference: "registry", want: config.KyvernoNamespace() + "/registry", cluster: true},
		{name: "cluster explicit namespace", reference: "team-b/registry", want: "team-b/registry", cluster: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			spec := policiesv1beta1.ImageValidatingPolicySpec{Credentials: &policiesv1beta1.Credentials{Secrets: []string{tc.reference}}}
			var policy policiesv1beta1.ImageValidatingPolicyLike = &policiesv1beta1.NamespacedImageValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "team-a"}, Spec: spec,
			}
			if tc.cluster {
				policy = &policiesv1beta1.ImageValidatingPolicy{Spec: spec}
			}
			original := policy.DeepCopyObject()
			lister := &recordingSecretLister{}
			compiled, errs := NewCompiler(lister).Compile(policy, nil)
			assert.Equal(t, original, policy, "compilation must not mutate cached policies")
			assert.Empty(t, lister.references, "compilation must not access secrets")
			if tc.denied {
				require.NotEmpty(t, errs)
				assert.Nil(t, compiled)
				return
			}
			require.Empty(t, errs)
			transport := &forbiddenRegistryTransport{}
			opts := append(compiled.(*compiledPolicy).authOpts, remote.WithTransport(transport))
			_, err := remote.Get(name.MustParseReference("192.0.2.1/test/image:latest"), opts...)
			require.ErrorContains(t, err, "stop at secret lookup")
			assert.Equal(t, []string{tc.want}, lister.references)
			assert.Zero(t, transport.calls)
		})
	}
}

func TestCompileRejectsNamespacedSignaturePullSecrets(t *testing.T) {
	t.Parallel()
	for _, reference := range []string{"team-b/signatures", "kyverno/signatures", "team-a/signatures/extra"} {
		t.Run(reference, func(t *testing.T) {
			t.Parallel()
			policy := &policiesv1beta1.NamespacedImageValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "team-a"},
				Spec: policiesv1beta1.ImageValidatingPolicySpec{Attestors: []policiesv1beta1.Attestor{{Name: "signer", Cosign: &policiesv1beta1.Cosign{
					Source: &policiesv1beta1.Source{SignaturePullSecrets: []corev1.LocalObjectReference{{Name: reference}}},
				}}}},
			}
			lister := &recordingSecretLister{}
			compiled, errs := NewCompiler(lister).Compile(policy, nil)
			require.NotEmpty(t, errs)
			assert.Nil(t, compiled)
			assert.Empty(t, lister.references)
			assert.Equal(t, "spec.attestors[0].cosign.source.PullSecrets", errs[0].Field)
		})
	}
}

func TestCompiledNamespacedPolicyUsesAuthorizedTenantSecretLookup(t *testing.T) {
	libs.GetLibsCtx()
	t.Parallel()
	for _, forbidden := range []bool{false, true} {
		t.Run(fmt.Sprint(forbidden), func(t *testing.T) {
			t.Parallel()
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			secret := func(namespace, user string) *corev1.Secret {
				return &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "registry"}, Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{"auths":{"192.0.2.1":{"username":"` + user + `","password":"test-only"}}}`)},
				}
			}
			require.NoError(t, indexer.Add(secret(config.KyvernoNamespace(), "administrator")))
			client := kubefake.NewSimpleClientset(secret("team-a", "tenant"))
			if forbidden {
				client.PrependReactor("get", "secrets", func(k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "secrets"}, "registry", errors.New("tenant read forbidden"))
				})
			}
			lister := kubeutils.WithSecretClient(context.Background(), corev1listers.NewSecretLister(indexer), client.CoreV1())
			policy := &policiesv1beta1.NamespacedImageValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: "team-a"},
				Spec: policiesv1beta1.ImageValidatingPolicySpec{Credentials: &policiesv1beta1.Credentials{Secrets: []string{"registry"}}},
			}
			compiled, errs := NewCompiler(lister).Compile(policy, nil)
			require.Empty(t, errs)
			transport := &forbiddenRegistryTransport{}
			opts := append(compiled.(*compiledPolicy).authOpts, remote.WithTransport(transport))
			_, err := remote.Get(name.MustParseReference("192.0.2.1/test/image:latest"), opts...)
			require.Error(t, err)
			require.Len(t, client.Actions(), 1)
			assert.Equal(t, "get", client.Actions()[0].GetVerb())
			assert.Equal(t, "team-a", client.Actions()[0].GetNamespace())
			if forbidden {
				assert.ErrorContains(t, err, "tenant read forbidden")
				assert.Zero(t, transport.calls)
			} else {
				assert.ErrorContains(t, err, "registry access is forbidden in this test")
				assert.Positive(t, transport.calls)
			}
		})
	}
}
