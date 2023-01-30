package api

import (
	"time"
)

// ExecutionStats stores the statistics for the single policy/rule application
type ExecutionStats struct {
	// ProcessingTime is the time required to apply the policy/rule on the resource
	ProcessingTime time.Duration
	// Timestamp of the instant the policy/rule got triggered
	Timestamp int64
}

// PolicyStats stores statistics for the single policy application
type PolicyStats struct {
	// ExecutionStats policy execution stats
	ExecutionStats
	// RulesAppliedCount is the count of rules that were applied successfully
	RulesAppliedCount int
	// RulesErrorCount is the count of rules that with execution errors
	RulesErrorCount int
}
