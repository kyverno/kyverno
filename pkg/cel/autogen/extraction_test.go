package autogen

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestResolveConfig(t *testing.T) {
	t.Run("built-in kind resolves via ConfigsMap, unchanged", func(t *testing.T) {
		got := ResolveConfig("deployments")
		assert.Equal(t, ConfigsMap["deployments"], got)
	})

	t.Run("explicit resource.version.group format resolves to an extraction target, no cluster needed", func(t *testing.T) {
		got := ResolveConfig("jobsets.v1alpha2.jobset.x-k8s.io")
		assert.Equal(t, &Config{
			Target: policiesv1beta1.Target{
				Group:    "jobset.x-k8s.io",
				Version:  "v1alpha2",
				Resource: "jobsets",
				Kind:     "Jobset",
			},
			ReplacementsRef: ExtractionReplacementsRef,
		}, got)
	})

	t.Run("malformed explicit format is ignored", func(t *testing.T) {
		assert.Nil(t, ResolveConfig("jobsets.v1alpha2"))
		assert.Nil(t, ResolveConfig(""))
	})

	t.Run("bare name with no BareNameResolver set is ignored", func(t *testing.T) {
		t.Cleanup(func() { BareNameResolver = nil })
		BareNameResolver = nil
		assert.Nil(t, ResolveConfig("jobsets"))
	})

	t.Run("bare name resolves via BareNameResolver when set", func(t *testing.T) {
		t.Cleanup(func() { BareNameResolver = nil })
		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "jobset.x-k8s.io", Version: "v1alpha2"}})
		mapper.Add(schema.GroupVersionKind{Group: "jobset.x-k8s.io", Version: "v1alpha2", Kind: "JobSet"}, meta.RESTScopeNamespace)
		BareNameResolver = RESTMapperBareNameResolver(mapper)

		got := ResolveConfig("jobsets")
		assert.Equal(t, &Config{
			Target: policiesv1beta1.Target{
				Group:    "jobset.x-k8s.io",
				Version:  "v1alpha2",
				Resource: "jobsets",
				Kind:     "JobSet",
			},
			ReplacementsRef: ExtractionReplacementsRef,
		}, got)
	})

	t.Run("unknown bare name via BareNameResolver is ignored", func(t *testing.T) {
		t.Cleanup(func() { BareNameResolver = nil })
		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "jobset.x-k8s.io", Version: "v1alpha2"}})
		mapper.Add(schema.GroupVersionKind{Group: "jobset.x-k8s.io", Version: "v1alpha2", Kind: "JobSet"}, meta.RESTScopeNamespace)
		BareNameResolver = RESTMapperBareNameResolver(mapper)

		assert.Nil(t, ResolveConfig("widgets"))
	})
}
