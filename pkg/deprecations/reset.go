package deprecations

// ClearExecutionEscapeHatch drops any registered escape hatch, restoring the zero state in which
// denial is unconditional.
//
// This exists so a test can assert what registering the hatch actually changed, which needs a known
// starting point. `go test -count=N` reuses one process, so without it the callback registered by
// the first iteration makes the second iteration's "not registered yet" assertion vacuous.
//
// It is not the unregister API the design deliberately omits. Removing one entry selectively would
// be a lever for widening the gate at runtime; clearing the hatch only ever makes the gate stricter.
//
// It deliberately leaves the subresource allowances alone. Those are registered from `init()` in the
// legacy_status.go files, which runs once per process, so clearing them would break sibling tests in
// the same package that cannot re-trigger the registration.
func ClearExecutionEscapeHatch() {
	executionEscapeHatch.Store(nil)
}
