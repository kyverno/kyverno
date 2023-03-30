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

func NewExecutionStats(timestamp time.Time) ExecutionStats {
	return ExecutionStats{
		Timestamp: timestamp.Unix(),
	}
}

func (s *ExecutionStats) Done(timestamp time.Time) {
	s.ProcessingTime = timestamp.Sub(time.Unix(s.Timestamp, 0))
}

// PolicyStats stores statistics for the single policy application
type PolicyStats struct {
	// RulesAppliedCount is the count of rules that were applied successfully
	RulesAppliedCount int
	// RulesErrorCount is the count of rules that with execution errors
	RulesErrorCount int
}
