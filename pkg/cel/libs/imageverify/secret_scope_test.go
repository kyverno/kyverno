package imageverify

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type imageLookupRecorder struct {
	imagedataloader.ImageContext
	calls int
}

func (r *imageLookupRecorder) Get(context.Context, string, []remote.Option, []name.Option) (*imagedataloader.ImageData, error) {
	r.calls++
	return nil, errors.New("stop at image lookup")
}

func signatureSecretAttestor(reference string) policiesv1beta1.Attestor {
	return policiesv1beta1.Attestor{Name: "signer", Cosign: &policiesv1beta1.Cosign{
		Source: &policiesv1beta1.Source{SignaturePullSecrets: []corev1.LocalObjectReference{{Name: reference}}},
	}}
}

func TestImageVerifyCELFuncsConfinesNamespacedSecrets(t *testing.T) {
	t.Parallel()
	for _, source := range []string{"credentials", "attestor"} {
		for _, reference := range []string{"team-b/registry", "kyverno/registry", "team-a/registry/extra"} {
			t.Run(source+"/"+reference, func(t *testing.T) {
				t.Parallel()
				policy := &policiesv1beta1.NamespacedImageValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: "team-a"}}
				if source == "credentials" {
					policy.Spec.Credentials = &policiesv1beta1.Credentials{Secrets: []string{reference}}
				} else {
					policy.Spec.Attestors = []policiesv1beta1.Attestor{signatureSecretAttestor(reference)}
				}
				original := policy.DeepCopy()
				images := &imageLookupRecorder{}
				impl, err := NewIvFuncs(logr.Discard(), policy, nil, types.DefaultTypeAdapter, nil)
				require.Error(t, err)
				assert.Nil(t, impl)
				assert.Zero(t, images.calls)
				assert.Equal(t, original, policy)
			})
		}
	}
}

func TestRuntimeAttestorSecretsAreScopedBeforeImageLookup(t *testing.T) {
	t.Parallel()
	for _, operation := range []string{"signature", "attestation"} {
		for _, tc := range []struct {
			name, reference string
			cluster, denied bool
		}{
			{name: "bare name", reference: "registry"},
			{name: "same namespace", reference: "team-a/registry"},
			{name: "foreign namespace", reference: "team-b/registry", denied: true},
			{name: "installation namespace", reference: "kyverno/registry", denied: true},
			{name: "malformed reference", reference: "team-a/registry/extra", denied: true},
			{name: "cluster foreign namespace", reference: "team-b/registry", cluster: true},
		} {
			t.Run(operation+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				spec := policiesv1beta1.ImageValidatingPolicySpec{Attestations: []policiesv1beta1.Attestation{{Name: "sbom"}}}
				var policy policiesv1beta1.ImageValidatingPolicyLike = &policiesv1beta1.NamespacedImageValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: "team-a"}, Spec: spec}
				if tc.cluster {
					policy = &policiesv1beta1.ImageValidatingPolicy{Spec: spec}
				}
				images := &imageLookupRecorder{}
				impl, err := NewIvFuncs(logr.Discard(), policy, nil, types.DefaultTypeAdapter, nil)
				require.NoError(t, err)
				impl = NewRuntimeForPolicy(impl, images, nil, NewImageVerificationResults()).functions
				attestor := signatureSecretAttestor(tc.reference)
				original := attestor.DeepCopy()
				attestors := types.NewDynamicList(types.DefaultTypeAdapter, []policiesv1beta1.Attestor{attestor})
				var result ref.Val
				if operation == "attestation" {
					result = impl.verify_image_attestations_string_string_stringarray(types.String("registry.example/image:latest"), types.String("sbom"), attestors)
				} else {
					result = impl.verify_image_signature_string_stringarray(types.String("registry.example/image:latest"), attestors)
				}
				require.True(t, types.IsError(result))
				if tc.denied {
					assert.Zero(t, images.calls, "reject before image/registry credential lookup")
					assert.Contains(t, result.Value().(error).Error(), "PullSecrets")
				} else {
					assert.Equal(t, 1, images.calls)
					assert.Contains(t, result.Value().(error).Error(), "stop at image lookup")
				}
				assert.Equal(t, original, &attestor, "runtime inputs must not be mutated")
			})
		}
	}
}
