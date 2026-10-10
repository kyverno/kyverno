package resource

import (
	"github.com/kyverno/kyverno/pkg/deprecations"
	"github.com/kyverno/kyverno/pkg/policycache"
	"github.com/kyverno/kyverno/pkg/toggle"
)

// registerLegacyExecutionEscapeHatch anchors the kyverno binary's 1.20 migration-grace escape
// hatch to the construction of legacy policy execution itself, not to the typed validation
// handlers: this binary can execute a legacy ClusterPolicy/Policy only through pCache, so once
// that parameter is gone (or nil), so is this registration's effect. Deleting pkg/policycache or
// the legacy branch that reads it removes the hatch in this binary with no ordering discipline
// required against #17710. See pkg/deprecations.RegisterLegacyExecutionEscapeHatch and design
// decision 6 for #17708.
func registerLegacyExecutionEscapeHatch(pCache policycache.Cache) {
	if pCache == nil {
		// A nil cache means this binary was constructed without legacy execution wired in
		// (e.g. a test double, or a #17710-era binary that dropped the parameter but kept the
		// call site during a refactor); registering an escape hatch here would honour a stale
		// toggle/env var with nothing to execute against, exactly the state this design
		// forbids. Register nothing.
		return
	}
	deprecations.RegisterLegacyExecutionEscapeHatch(func() bool {
		return !toggle.BlockLegacyPolicyAPIs.Enabled()
	})
}
