package local

import (
	"context"
	"fmt"
	"sync"

	"github.com/kyverno/kyverno/pkg/image/verifiers"
)

// Provider holds local attestations supplied out-of-band (e.g. by the Kyverno CLI
// during `kyverno test`) so that verifyImages.attestations rules can be evaluated
// without fetching attestations from an OCI registry.
type Provider struct {
	mu sync.RWMutex
	// statements is keyed by "image|predicateType" and holds in-toto statements.
	statements map[string][]map[string]any
}

// NewProvider creates an empty local attestation provider.
func NewProvider() *Provider {
	return &Provider{
		statements: make(map[string][]map[string]any),
	}
}

func key(image, predicateType string) string {
	return fmt.Sprintf("%s|%s", image, predicateType)
}

// Add registers a local predicate for the given image and predicate type. The
// predicate is wrapped into a minimal in-toto statement that mirrors what the
// cosign/notary verifiers decode from registry attestations.
func (p *Provider) Add(image, predicateType string, predicate map[string]any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	statement := map[string]any{
		"type":          predicateType,
		"predicateType": predicateType,
		"predicate":     predicate,
	}
	k := key(image, predicateType)
	p.statements[k] = append(p.statements[k], statement)
}

// Get returns the local statements for the given image and predicate type.
func (p *Provider) Get(image, predicateType string) ([]map[string]any, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	statements, ok := p.statements[key(image, predicateType)]
	return statements, ok
}

// Has reports whether any local statements exist for the given image and predicate type.
func (p *Provider) Has(image, predicateType string) bool {
	_, ok := p.Get(image, predicateType)
	return ok
}

type contextKey struct{}

// WithProvider returns a copy of ctx carrying the local attestation provider.
func WithProvider(ctx context.Context, provider *Provider) context.Context {
	return context.WithValue(ctx, contextKey{}, provider)
}

// ProviderFromContext retrieves the local attestation provider from ctx, if any.
func ProviderFromContext(ctx context.Context) (*Provider, bool) {
	provider, ok := ctx.Value(contextKey{}).(*Provider)
	return provider, ok
}

// verifier is a verifiers.ImageVerifier that serves attestations from a local Provider.
type verifier struct {
	provider *Provider
}

// NewVerifier returns a verifiers.ImageVerifier backed by the given local provider.
func NewVerifier(provider *Provider) verifiers.ImageVerifier {
	return &verifier{provider: provider}
}

// VerifySignature is not supported for local attestations.
func (v *verifier) VerifySignature(_ context.Context, opts verifiers.Options) (*verifiers.Response, error) {
	return nil, fmt.Errorf("signature verification is not supported for local attestations")
}

// FetchAttestations returns the local statements matching the requested image and predicate type.
func (v *verifier) FetchAttestations(_ context.Context, opts verifiers.Options) (*verifiers.Response, error) {
	statements, ok := v.provider.Get(opts.ImageRef, opts.Type)
	if !ok {
		return nil, fmt.Errorf("no local attestations found for image %q with predicate type %q", opts.ImageRef, opts.Type)
	}
	return &verifiers.Response{Statements: statements}, nil
}
