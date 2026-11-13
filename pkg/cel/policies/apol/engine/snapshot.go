package engine

import (
	"context"
	"errors"
	"slices"
	"strings"
)

// ErrNoSnapshot is returned by a Provider that has no policy snapshot yet, for
// example before the policy cache has synced. The engine fails closed on it.
var ErrNoSnapshot = errors.New("authorizing policy snapshot is not ready")

// Snapshot is one immutable view of every compiled AuthorizingPolicy.
//
// A snapshot is built once per policy-set change (reconcile) and shared by all
// concurrent authorization requests. The engine reads a single snapshot per
// request, so a request never observes a half-applied policy update. Policies
// are kept sorted by name only so that diagnostics (reasons and evaluation
// errors) are deterministic; the decision never depends on the order.
type Snapshot struct {
	generation uint64
	policies   []CompiledPolicy
}

// NewSnapshot returns a snapshot of the given policies. The slice is copied;
// callers may reuse it.
func NewSnapshot(generation uint64, policies ...CompiledPolicy) *Snapshot {
	out := make([]CompiledPolicy, 0, len(policies))
	for _, p := range policies {
		if p != nil && p.Policy() != nil {
			out = append(out, p)
		}
	}
	slices.SortStableFunc(out, func(a, b CompiledPolicy) int {
		return strings.Compare(a.Policy().Name, b.Policy().Name)
	})
	return &Snapshot{generation: generation, policies: out}
}

// Generation identifies the snapshot. It increases with every policy-set change
// in one provider and is used in logs only.
func (s *Snapshot) Generation() uint64 { return s.generation }

// Len returns the number of policies in the snapshot.
func (s *Snapshot) Len() int { return len(s.policies) }

// Policies returns a copy of the policies in the snapshot, sorted by name.
func (s *Snapshot) Policies() []CompiledPolicy { return slices.Clone(s.policies) }

// Provider returns the current policy snapshot.
type Provider interface {
	// Snapshot returns the current immutable snapshot, or ErrNoSnapshot when the
	// provider is not ready yet.
	Snapshot(ctx context.Context) (*Snapshot, error)
}

// ProviderFunc adapts a function to a Provider.
type ProviderFunc func(context.Context) (*Snapshot, error)

// Snapshot implements Provider.
func (f ProviderFunc) Snapshot(ctx context.Context) (*Snapshot, error) { return f(ctx) }

// StaticProvider returns a Provider that always serves one snapshot of the given policies.
func StaticProvider(policies ...CompiledPolicy) Provider {
	snapshot := NewSnapshot(1, policies...)
	return ProviderFunc(func(context.Context) (*Snapshot, error) { return snapshot, nil })
}
