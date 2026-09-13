package policycache

import (
	"encoding/json"
	"fmt"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubecache "k8s.io/client-go/tools/cache"
)

// benchPolicyRaw returns a validate ClusterPolicy with a failureActionOverride
// (so GetPolicies exercises checkValidationFailureActionOverrides), matching
// the given kind. name must be unique per seeded policy.
func benchPolicyRaw(name, kind string) []byte {
	return []byte(fmt.Sprintf(`{
		"metadata": {
			"name": %q
		},
		"spec": {
			"background": false,
			"rules": [
				{
					"match": {
						"resources": {
							"kinds": [%q]
						}
					},
					"name": "check-label-app",
					"validate": {
						"message": "The label 'app' is required.",
						"pattern": {
							"metadata": {
								"labels": {
									"app": "?*"
								}
							}
						}
					}
				}
			],
			"validationFailureAction": "enforce",
			"validationFailureActionOverrides": [
				{
					"action": "enforce",
					"namespaces": ["default"]
				},
				{
					"action": "audit",
					"namespaces": ["test"]
				}
			]
		}
	}`, name, kind))
}

func benchPolicy(b *testing.B, name, kind string) *kyvernov1.ClusterPolicy {
	b.Helper()
	var policy *kyvernov1.ClusterPolicy
	if err := json.Unmarshal(benchPolicyRaw(name, kind), &policy); err != nil {
		b.Fatalf("failed to unmarshal bench policy: %v", err)
	}
	return policy
}

// seedCache populates a cache with n total policies. Only one policy targets
// Pod (the GVR the benchmark looks up); the remaining n-1 policies target
// Namespace, a GVR that is never queried in this benchmark.
//
// This seeding is deliberate: GetPolicies (and the underlying policyMap.get)
// iterate only over the set of policy names stored under the queried
// (PolicyType, GVR) key, i.e. cost is O(matched-for-this-GVR), not
// O(stored-total). Seeding N pod-matching policies would make the benchmark
// scale with N and defeat the purpose of an N-indexed ceiling (it would
// always regress "correctly" as N grows, telling us nothing about whether an
// unrelated change made lookup cost scale with total cache size). By holding
// the pod-matched count constant at 1 and growing only the unrelated
// (Namespace-matched) population, the policies-1/10/100/1000 ceilings should
// be flat at baseline; a future change that makes GetPolicies scale with
// total stored-policy count (not just matched count) would break the
// policies-1000 ceiling first.
func seedCache(b *testing.B, n int) Cache {
	b.Helper()
	cache := NewCache()
	finder := TestResourceFinder{}

	for i := 0; i < n-1; i++ {
		policy := benchPolicy(b, fmt.Sprintf("ns-policy-%d", i), "Namespace")
		key, _ := kubecache.MetaNamespaceKeyFunc(policy)
		if err := cache.Set(key, policy, finder); err != nil {
			b.Fatalf("failed to seed namespace policy: %v", err)
		}
	}

	podPolicy := benchPolicy(b, "pod-policy", "Pod")
	key, _ := kubecache.MetaNamespaceKeyFunc(podPolicy)
	if err := cache.Set(key, podPolicy, finder); err != nil {
		b.Fatalf("failed to seed pod policy: %v", err)
	}

	return cache
}

// BenchmarkGetPolicies measures cache lookup cost as stored-policy count
// grows, holding the count of policies matching the queried GVR (Pod) fixed
// at 1 (see seedCache). Each sub-benchmark exercises both hot admission
// lookups (ValidateEnforce and ValidateAudit; the latter also traverses the
// failure-action-override path via checkValidationFailureActionOverrides).
func BenchmarkGetPolicies(b *testing.B) {
	testNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
	}

	for _, n := range []int{1, 10, 100, 1000} {
		b.Run(fmt.Sprintf("policies-%d", n), func(b *testing.B) {
			cache := seedCache(b, n)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = cache.GetPolicies(ValidateEnforce, podsGVR, "", testNamespace)
				_ = cache.GetPolicies(ValidateAudit, podsGVR, "", testNamespace)
			}
		})
	}
}
