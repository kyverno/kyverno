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
				// No KindResolver set (no cluster): falls back to the
				// best-effort guess, which gets this irregular plural wrong.
				Kind: "Jobset",
			},
			ReplacementsRef: ExtractionReplacementsRef,
		}, got)
	})

	t.Run("explicit resource.version.group format's guessed Kind is corrected by KindResolver when set", func(t *testing.T) {
		t.Cleanup(func() { KindResolver = nil })
		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "jobset.x-k8s.io", Version: "v1alpha2"}})
		mapper.Add(schema.GroupVersionKind{Group: "jobset.x-k8s.io", Version: "v1alpha2", Kind: "JobSet"}, meta.RESTScopeNamespace)
		KindResolver = RESTMapperKindResolver(mapper)

		got := ResolveConfig("jobsets.v1alpha2.jobset.x-k8s.io")
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

	t.Run("kind name does not resolve via discovery, only the plural resource name does", func(t *testing.T) {
		t.Cleanup(func() { BareNameResolver = nil })
		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "jobset.x-k8s.io", Version: "v1alpha2"}})
		mapper.Add(schema.GroupVersionKind{Group: "jobset.x-k8s.io", Version: "v1alpha2", Kind: "JobSet"}, meta.RESTScopeNamespace)
		BareNameResolver = RESTMapperBareNameResolver(mapper)

		// Discovery matches resource names case-insensitively, so "JobSet"
		// resolves to the kind; the resource name itself is not "JobSet".
		assert.Nil(t, ResolveConfig("JobSet"))
		assert.NotNil(t, ResolveConfig("jobsets"))
	})

	t.Run("unknown bare name via BareNameResolver is ignored", func(t *testing.T) {
		t.Cleanup(func() { BareNameResolver = nil })
		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "jobset.x-k8s.io", Version: "v1alpha2"}})
		mapper.Add(schema.GroupVersionKind{Group: "jobset.x-k8s.io", Version: "v1alpha2", Kind: "JobSet"}, meta.RESTScopeNamespace)
		BareNameResolver = RESTMapperBareNameResolver(mapper)

		assert.Nil(t, ResolveConfig("widgets"))
	})

	// Offline: no BareNameResolver, so only the canonical explicit form and
	// the built-in plurals can resolve. A non-canonical resource name in
	// either casing must be rejected rather than generating a rule that can
	// never match (see #18833).
	t.Run("explicit forms", func(t *testing.T) {
		t.Cleanup(func() { BareNameResolver = nil })
		BareNameResolver = nil

		tests := []struct {
			name string
			want *Config
		}{
			{name: "Deployment.v1.apps"},
			{name: "deployment.v1.apps"},
			{name: "Jobsets.v1alpha2.jobset.x-k8s.io"},
			{
				name: "deployments.v1.apps",
				want: &Config{
					Target: policiesv1beta1.Target{
						Group:    "apps",
						Version:  "v1",
						Resource: "deployments",
						Kind:     "Deployment",
					},
					ReplacementsRef: ExtractionReplacementsRef,
				},
			},
			{
				name: "jobsets.v1alpha2.jobset.x-k8s.io",
				want: &Config{
					Target: policiesv1beta1.Target{
						Group:    "jobset.x-k8s.io",
						Version:  "v1alpha2",
						Resource: "jobsets",
						Kind:     "Jobset",
					},
					ReplacementsRef: ExtractionReplacementsRef,
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				assert.Equal(t, tt.want, ResolveConfig(tt.name))
			})
		}
	})

	t.Run("wildcard resource or group is not a concrete target", func(t *testing.T) {
		t.Cleanup(func() { BareNameResolver = nil })
		BareNameResolver = nil
		// Only the API version may be a wildcard: "*" as the resource or the
		// group would generate a rule matching unrelated resources or groups.
		assert.Nil(t, ResolveConfig("deployments.v1.*"))
		assert.Nil(t, ResolveConfig("*.v1.example.com"))
		assert.Nil(t, ResolveConfig("*.v1.*"))
	})

	t.Run("subresource and slash syntax in resource or group is rejected", func(t *testing.T) {
		t.Cleanup(func() { BareNameResolver = nil })
		BareNameResolver = nil
		// Subresource syntax like "*/status" or "deployments/status" is not a
		// plural resource name and would generate rules matching unrelated
		// resources. Any resource or group component containing "/" or "*" is
		// rejected regardless of the other components.
		rejectCases := []string{
			// subresource syntax in the resource segment
			"*/status.v1.apps",
			"*/scale.v1.apps",
			"deployments/status.v1.apps",
			"deployments/scale.v1.apps",
			// wildcard in the resource segment (beyond bare "*")
			"deploy*.v1.apps",
			// slash in the group segment
			"deployments.v1.example.com/v1",
			// wildcard within the group segment
			"deployments.v1.example*.com",
			// bare "*" resource and group (re-validated here)
			"*.v1.apps",
			"deployments.v1.*",
		}
		for _, name := range rejectCases {
			assert.Nil(t, ResolveConfig(name), "expected nil for %q", name)
		}
		// Valid concrete targets must continue to resolve.
		assert.NotNil(t, ResolveConfig("deployments.v1.apps"), "valid built-in must resolve")
		assert.NotNil(t, ResolveConfig("deployments.*.apps"), "wildcard version must resolve")
		assert.NotNil(t, ResolveConfig("cronjobs.v1beta1.batch"), "historical version must resolve")
		assert.NotNil(t, ResolveConfig("jobsets.v1alpha2.jobset.x-k8s.io"), "custom resource must resolve")
	})
}

// TestResolveMutatingConfig covers the MutatingPolicy-specific resolution: an
// explicit canonical form naming a built-in controller must land on the same
// ConfigsMap entry as its bare name, because the MutatingPolicy engine skips
// extraction-mode configs. Custom resources keep their extraction target.
func TestResolveMutatingConfig(t *testing.T) {
	t.Cleanup(func() {
		BareNameResolver = nil
		KindResolver = nil
	})
	BareNameResolver = nil
	KindResolver = nil

	tests := []struct {
		name string
		want *Config
	}{
		{name: "Deployment.v1.apps"},
		{name: "deployment.v1.apps"},
		{name: "Jobsets.v1alpha2.jobset.x-k8s.io"},
		{name: "deployments", want: ConfigsMap["deployments"]},
		{name: "deployments.v1.apps", want: ConfigsMap["deployments"]},
		{name: "jobs.v1.batch", want: ConfigsMap["jobs"]},
		{
			// The wildcard version keeps its target version so the generated
			// resource rule matches every version, while the built-in
			// replacement configuration still applies to the pod template.
			name: "deployments.*.apps",
			want: &Config{
				Target: policiesv1beta1.Target{
					Group:    "apps",
					Version:  "*",
					Resource: "deployments",
					Kind:     "Deployment",
				},
				ReplacementsRef: AutogenDefaults,
			},
		},
		{
			// A historical version is concrete, so it keeps exact-version
			// matching; only the wildcard has to match every version.
			name: "cronjobs.v1beta1.batch",
			want: &Config{
				Target: policiesv1beta1.Target{
					Group:    "batch",
					Version:  "v1beta1",
					Resource: "cronjobs",
					Kind:     "CronJob",
				},
				ReplacementsRef: AutogenCronjobs,
			},
		},
		{
			// Wildcard CronJob versions must retain the wildcard target while
			// using the canonical built-in Kind and CronJob template mapping.
			name: "cronjobs.*.batch",
			want: &Config{
				Target: policiesv1beta1.Target{
					Group:    "batch",
					Version:  "*",
					Resource: "cronjobs",
					Kind:     "CronJob",
				},
				ReplacementsRef: AutogenCronjobs,
			},
		},
		{
			// A built-in resource at a version it does not have is not
			// the built-in config, so no rule is generated for it.
			name: "deployments.v2.apps",
		},
		{
			// Custom workload CRDs keep their own extraction target so
			// their existing (unsupported) mutation path is unchanged.
			name: "jobsets.v1alpha2.jobset.x-k8s.io",
			want: &Config{
				Target: policiesv1beta1.Target{
					Group:    "jobset.x-k8s.io",
					Version:  "v1alpha2",
					Resource: "jobsets",
					Kind:     "Jobset",
				},
				ReplacementsRef: ExtractionReplacementsRef,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ResolveMutatingConfig(tt.name))
		})
	}
}

// TestResolveConfigVersionCompatibility covers the accepted API versions for a
// built-in controller's explicit form. A wildcard version and the versions the
// built-in groups actually served must keep resolving (they matched before the
// version check was added), while a version the resource never had must not.
func TestResolveConfigVersionCompatibility(t *testing.T) {
	t.Cleanup(func() {
		BareNameResolver = nil
		KindResolver = nil
	})
	BareNameResolver = nil
	KindResolver = nil

	tests := []struct {
		name string
		want *Config
	}{
		{
			// apiVersions: ["*"]: the target keeps the wildcard version.
			name: "deployments.*.apps",
			want: &Config{
				Target: policiesv1beta1.Target{
					Group:    "apps",
					Version:  "*",
					Resource: "deployments",
					Kind:     "Deployment",
				},
				ReplacementsRef: ExtractionReplacementsRef,
			},
		},
		{
			// Historical CronJob version, still served as batch/v1beta1.
			name: "cronjobs.v1beta1.batch",
			want: &Config{
				Target: policiesv1beta1.Target{
					Group:    "batch",
					Version:  "v1beta1",
					Resource: "cronjobs",
					Kind:     "CronJob",
				},
				ReplacementsRef: ExtractionReplacementsRef,
			},
		},
		{
			name: "statefulsets.v1beta2.apps",
			want: &Config{
				Target: policiesv1beta1.Target{
					Group:    "apps",
					Version:  "v1beta2",
					Resource: "statefulsets",
					Kind:     "StatefulSet",
				},
				ReplacementsRef: ExtractionReplacementsRef,
			},
		},
		{
			name: "daemonsets.v1beta1.apps",
			want: &Config{
				Target: policiesv1beta1.Target{
					Group:    "apps",
					Version:  "v1beta1",
					Resource: "daemonsets",
					Kind:     "DaemonSet",
				},
				ReplacementsRef: ExtractionReplacementsRef,
			},
		},
		{
			name: "daemonsets.v1beta2.apps",
			want: &Config{
				Target: policiesv1beta1.Target{
					Group:    "apps",
					Version:  "v1beta2",
					Resource: "daemonsets",
					Kind:     "DaemonSet",
				},
				ReplacementsRef: ExtractionReplacementsRef,
			},
		},
		{
			name: "replicasets.v1beta1.apps",
			want: &Config{
				Target: policiesv1beta1.Target{
					Group:    "apps",
					Version:  "v1beta1",
					Resource: "replicasets",
					Kind:     "ReplicaSet",
				},
				ReplacementsRef: ExtractionReplacementsRef,
			},
		},
		{
			name: "replicasets.v1beta2.apps",
			want: &Config{
				Target: policiesv1beta1.Target{
					Group:    "apps",
					Version:  "v1beta2",
					Resource: "replicasets",
					Kind:     "ReplicaSet",
				},
				ReplacementsRef: ExtractionReplacementsRef,
			},
		},
		{
			// A version the built-in resource never had.
			name: "deployments.v2.apps",
		},
		{
			// Jobs were always batch/v1, so a beta version is not a
			// historical version of this resource.
			name: "jobs.v1beta1.batch",
		},
		{
			// The resource and group must identify the same built-in: a
			// deployment in batch is not the apps Deployment, so it must not
			// borrow that config's Kind or replacement path.
			name: "deployments.v1.batch",
		},
		{
			name: "cronjobs.v1.apps",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ResolveConfig(tt.name))
		})
	}
}
