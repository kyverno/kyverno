package framework

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"

	gpolengine "github.com/kyverno/kyverno/pkg/cel/policies/gpol/engine"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	vpolengine "github.com/kyverno/kyverno/pkg/cel/policies/vpol/engine"
	kyvernov1beta1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/policies.kyverno.io/v1beta1"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
)

// PolicyType identifies a Kyverno CEL policy type that the framework can wire
// into a TestEnv. Callers select the types they need via WithPolicyTypes.
type PolicyType int

const (
	Vpol PolicyType = iota + 1
	Mpol
	Gpol
	Dpol
)

func (p PolicyType) String() string {
	switch p {
	case Vpol:
		return "Vpol"
	case Mpol:
		return "Mpol"
	case Gpol:
		return "Gpol"
	case Dpol:
		return "Dpol"
	}
	return fmt.Sprintf("PolicyType(%d)", int(p))
}

// VpolSetup holds the per-test wiring for ValidatingPolicy.
type VpolSetup struct {
	Engine   vpolengine.Engine
	Provider vpolengine.Provider
}

// MpolSetup holds the per-test wiring for MutatingPolicy.
type MpolSetup struct {
	Engine   mpolengine.Engine
	Provider mpolengine.Provider
}

// GpolSetup holds the per-test wiring for GeneratingPolicy.
type GpolSetup struct {
	Engine           gpolengine.Engine
	Provider         gpolengine.Provider
	Lister           policiesv1beta1listers.GeneratingPolicyLister
	NamespacedLister policiesv1beta1listers.NamespacedGeneratingPolicyLister
}

// DpolSetup holds the per-test wiring for DeletingPolicy. PolexInformer is
// non-nil only when WithExceptionsEnabled was passed.
type DpolSetup struct {
	Deps          *DpolDeps
	PolexInformer kyvernov1beta1informers.PolicyExceptionInformer
}

// SetupOption configures a TestEnv built via NewTestEnvWithOptions.
type SetupOption func(*setupConfig)

type setupConfig struct {
	crdPaths     []string
	policyTypes  []PolicyType
	polexEnabled bool
}

// WithPolicyTypes selects which policy types to wire into the TestEnv. Calling
// this multiple times is additive. Duplicates are ignored.
func WithPolicyTypes(types ...PolicyType) SetupOption {
	return func(c *setupConfig) {
		c.policyTypes = append(c.policyTypes, types...)
	}
}

// WithExceptionsEnabled wires PolicyException support into every policy type
// that supports exceptions. Without this option, engines are built without
// exception listers (matching the *Engine non-Exceptions helpers).
func WithExceptionsEnabled() SetupOption {
	return func(c *setupConfig) { c.polexEnabled = true }
}

// WithCRDPaths adds CRD directories for envtest to load. Calls are additive:
// every path passed (across one or more invocations) is appended to the list.
// If WithCRDPaths is not used at all, the framework falls back to a built-in
// default that points at the policies.kyverno.io CRD directory, resolved
// relative to this source file so the path works regardless of the caller's
// cwd. Once any path is supplied via this option, the default is no longer
// used; callers wanting both must include the default path explicitly.
func WithCRDPaths(paths ...string) SetupOption {
	return func(c *setupConfig) {
		c.crdPaths = append(c.crdPaths, paths...)
	}
}

// NewTestEnvWithOptions builds a TestEnv with only the requested policy types
// wired in. The returned TestEnv exposes each requested type as a non-nil
// *<Type>Setup field; types that were not requested remain nil.
//
// The caller must still invoke (*TestEnv).Start before running tests and
// (*TestEnv).Stop when done, identical to the existing NewTestEnv flow.
func NewTestEnvWithOptions(opts ...SetupOption) (*TestEnv, error) {
	cfg := setupConfig{}
	for _, o := range opts {
		o(&cfg)
	}

	crdPaths := cfg.crdPaths
	if len(crdPaths) == 0 {
		crdPaths = []string{defaultCRDPath()}
	}

	env, err := NewTestEnv(crdPaths...)
	if err != nil {
		return nil, err
	}

	optCtx, optCancel := context.WithCancel(context.Background())
	env.optCancel = optCancel

	wired := map[PolicyType]bool{}
	for _, pt := range cfg.policyTypes {
		if wired[pt] {
			continue
		}
		if err := wirePolicyType(optCtx, env, pt, cfg.polexEnabled); err != nil {
			optCancel()
			env.Stop()
			return nil, fmt.Errorf("wiring %s: %w", pt, err)
		}
		wired[pt] = true
	}

	return env, nil
}

// wirePolicyType builds and attaches the per-type setup for pt. Each branch
// delegates to the existing per-type helpers so callers of those helpers see
// no behavior change.
func wirePolicyType(ctx context.Context, env *TestEnv, pt PolicyType, polexEnabled bool) error {
	switch pt {
	case Vpol:
		var (
			engine   vpolengine.Engine
			provider vpolengine.Provider
			err      error
		)
		if polexEnabled {
			engine, provider, err = NewVpolEngineWithExceptions(env.Mgr)
		} else {
			engine, provider, err = NewVpolEngine(env.Mgr)
		}
		if err != nil {
			return err
		}
		env.Vpol = &VpolSetup{Engine: engine, Provider: provider}

	case Mpol:
		var (
			engine   mpolengine.Engine
			provider mpolengine.Provider
			err      error
		)
		if polexEnabled {
			engine, provider, err = NewMpolEngineWithExceptions(ctx, env.Mgr, env.KubeClient, env.KyvernoClient, env.ContextProvider)
		} else {
			engine, provider, err = NewMpolEngine(ctx, env.Mgr, env.KubeClient, env.ContextProvider)
		}
		if err != nil {
			return err
		}
		env.Mpol = &MpolSetup{Engine: engine, Provider: provider}

	case Gpol:
		gpolLister, ngpolLister := NewGpolListers(ctx, env.KyvernoClient)
		if polexEnabled {
			polexLister := NewGpolPolexLister(ctx, env.KyvernoClient)
			gpolEngine, gpolProvider := NewGpolEngineWithExceptions(gpolLister, ngpolLister, polexLister)
			env.Gpol = &GpolSetup{
				Engine:           gpolEngine,
				Provider:         gpolProvider,
				Lister:           gpolLister,
				NamespacedLister: ngpolLister,
			}
		} else {
			gpolEngine, gpolProvider := NewGpolEngine(gpolLister, ngpolLister)
			env.Gpol = &GpolSetup{
				Engine:           gpolEngine,
				Provider:         gpolProvider,
				Lister:           gpolLister,
				NamespacedLister: ngpolLister,
			}
		}

	case Dpol:
		if polexEnabled {
			deps, polexInformer := NewDpolDepsWithExceptions(ctx, env.DClient, env.KyvernoClient, env.KubeClient, env.Mgr.GetRESTMapper(), env.ContextProvider)
			env.Dpol = &DpolSetup{Deps: deps, PolexInformer: polexInformer}
		} else {
			deps := NewDpolDeps(ctx, env.DClient, env.KyvernoClient, env.KubeClient, env.Mgr.GetRESTMapper(), env.ContextProvider)
			env.Dpol = &DpolSetup{Deps: deps}
		}

	default:
		return fmt.Errorf("unknown policy type: %s", pt)
	}
	return nil
}

// defaultCRDPath returns the policies.kyverno.io CRD directory resolved
// relative to this source file, so the path is stable regardless of the
// caller's working directory.
func defaultCRDPath() string {
	_, file, _, _ := runtime.Caller(0) //nolint:dogsled // runtime.Caller returns 4 values; only file is needed here
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "config", "crds", "policies.kyverno.io")
}
