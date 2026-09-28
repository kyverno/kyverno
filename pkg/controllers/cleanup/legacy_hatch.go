package cleanup

import (
	"github.com/kyverno/kyverno/pkg/deprecations"
	"github.com/kyverno/kyverno/pkg/toggle"
)

// registerLegacyExecutionEscapeHatch anchors the cleanup binary's 1.20 migration-grace escape
// hatch to the construction of this package's controller: pkg/controllers/cleanup is the legacy
// CleanupPolicy/ClusterCleanupPolicy scheduler and nothing else (CEL deletion lives in
// pkg/controllers/deleting), so deleting or unwiring NewController is cleanup execution removal
// and removes this registration with it. See pkg/deprecations.RegisterLegacyExecutionEscapeHatch
// and design decision 6 for #17708.
//
// Residual, stated plainly: cleanup.NewController is only ever called inside the leader-election
// callback in cmd/cleanup-controller/main.go, after the webhook server has already started. This
// registration is therefore leader-scoped and post-election: a non-leader replica, and a leader
// before its first election completes, deny legacy cleanup-policy writes even with the toggle
// disabled. That is fail-closed and intentional, not a gap to close here.
func registerLegacyExecutionEscapeHatch() {
	deprecations.RegisterLegacyExecutionEscapeHatch(func() bool {
		return !toggle.BlockLegacyPolicyAPIs.Enabled()
	})
}
