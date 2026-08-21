package auth

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/auth"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
)

// AuthChecks provides methods to performing operations on resource
type AuthChecks interface {
	// User returns the subject
	User() string
	// CanI returns 'true' if user has permissions for all specified verbs.
	// When the result is 'false' a message with details on missing verbs is returned.
	CanI(ctx context.Context, verbs []string, gvk, namespace, name, subresource string) (bool, string, error)
}

// Cache memoizes the outcome of SubjectAccessReview checks so that a single
// policy validation doesn't repeat identical, sequential API calls to the
// Kubernetes API server. Policies with many rules matching the same resource
// kind(s) would otherwise issue one SubjectAccessReview per verb, per rule,
// even though the (user, verb, gvk, namespace, name, subresource) tuple is
// identical across rules. A Cache is meant to be shared across all the Auth
// instances created while validating a single policy, and discarded afterwards.
type Cache struct {
	mu      sync.Mutex
	results map[cacheKey]cacheResult
}

type cacheKey struct {
	user        string
	verb        string
	gvk         string
	namespace   string
	name        string
	subresource string
}

type cacheResult struct {
	allowed bool
}

// NewCache returns a new, empty Cache
func NewCache() *Cache {
	return &Cache{results: map[cacheKey]cacheResult{}}
}

func (c *Cache) get(key cacheKey) (cacheResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	result, ok := c.results[key]
	return result, ok
}

func (c *Cache) set(key cacheKey, result cacheResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.results[key] = result
}

// Auth provides implementation to check if caller/self/kyverno has access to perofrm operations
type Auth struct {
	client dclient.Interface
	user   string
	log    logr.Logger
	cache  *Cache
}

// NewAuth returns a new instance of Auth for operations. The cache is optional
// (a nil cache disables memoization) and, when provided, should be shared by
// every Auth checking permissions on behalf of the same policy validation so
// that repeated checks across rules are only sent to the API server once.
func NewAuth(client dclient.Interface, user string, log logr.Logger, cache *Cache) *Auth {
	a := Auth{
		client: client,
		user:   user,
		log:    log,
		cache:  cache,
	}
	return &a
}

func (a *Auth) User() string {
	return a.user
}

func (a *Auth) CanI(ctx context.Context, verbs []string, gvk, namespace, name, subresource string) (bool, string, error) {
	var failedVerbs []string
	for _, v := range verbs {
		if ok, err := a.check(ctx, v, gvk, namespace, name, subresource); err != nil {
			return false, "", err
		} else if !ok {
			failedVerbs = append(failedVerbs, v)
		}
	}

	if len(failedVerbs) > 0 {
		msg := buildMessage(gvk, subresource, failedVerbs, a.user, namespace)
		return false, msg, nil
	}

	return true, "", nil
}

// CanICreate returns 'true' if self can 'create' resource
func (a *Auth) CanICreate(ctx context.Context, gvk, namespace, name, subresource string) (bool, error) {
	return a.check(ctx, "create", gvk, namespace, name, subresource)
}

func (a *Auth) check(ctx context.Context, verb, gvk, namespace, name, subresource string) (bool, error) {
	if a.cache == nil {
		return a.runAccessCheck(ctx, verb, gvk, namespace, name, subresource)
	}

	key := cacheKey{user: a.user, verb: verb, gvk: gvk, namespace: namespace, name: name, subresource: subresource}
	if result, ok := a.cache.get(key); ok {
		return result.allowed, nil
	}

	allowed, err := a.runAccessCheck(ctx, verb, gvk, namespace, name, subresource)
	if err != nil {
		// don't cache errors: they're often transient (e.g. a slow API server) and
		// should be retried rather than replayed for the remainder of the validation
		return false, err
	}
	a.cache.set(key, cacheResult{allowed: allowed})
	return allowed, nil
}

func (a *Auth) runAccessCheck(ctx context.Context, verb, gvk, namespace, name, subresource string) (bool, error) {
	subjectReview := a.client.GetKubeClient().AuthorizationV1().SubjectAccessReviews()
	canI := auth.NewCanI(a.client.Discovery(), subjectReview, gvk, namespace, name, verb, subresource, a.user)
	ok, _, err := canI.RunAccessCheck(ctx)
	if err != nil {
		return false, err
	}
	return ok, nil
}

func buildMessage(gvk string, subresource string, failedVerbs []string, user string, namespace string) string {
	resource := gvk
	if subresource != "" {
		resource = gvk + "/" + subresource
	}

	permissions := strings.Join(failedVerbs, ",")
	msg := fmt.Sprintf("%s requires permissions %s for resource %s", user, permissions, resource)
	if namespace != "" {
		msg = fmt.Sprintf("%s in namespace %s", msg, namespace)
	}
	return msg
}
