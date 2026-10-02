package processor

// InNamespaceScope reports whether a policy applies to a resource in the given namespace.
// A cluster-scoped policy applies everywhere, and a namespaced policy only applies to resources
// in its own namespace, which is how the admission webhooks and controllers scope them in a cluster.
func InNamespaceScope[T interface{ GetNamespace() string }](namespace string) func(T) bool {
	return func(policy T) bool {
		return policy.GetNamespace() == "" || policy.GetNamespace() == namespace
	}
}
