package api

import (
	"time"
)

// ExecutionStats stores the statistics for the single policy/rule application
type ExecutionStats struct {
	// processingTime is the time required to apply the policy/rule on the resource
	processingTime time.Duration
	// timestamp of the instant the policy/rule got triggered
	timestamp time.Time
}

func NewExecutionStats(startTime, endTime time.Time) ExecutionStats {
	return ExecutionStats{
		timestamp:      startTime,
		processingTime: endTime.Sub(startTime),
	}
}

func (s ExecutionStats) Time() time.Time {
	return s.timestamp
}

func (s ExecutionStats) Timestamp() int64 {
	return s.timestamp.Unix()
}

func (s ExecutionStats) ProcessingTime() time.Duration {
	return s.processingTime
}

// PolicyStats stores statistics for the single policy application
type PolicyStats struct {
	// rulesAppliedCount is the count of rules that were applied successfully
	rulesAppliedCount int
	// rulesErrorCount is the count of rules that with execution errors
	rulesErrorCount int
}

func (ps *PolicyStats) RulesAppliedCount() int {
	return ps.rulesAppliedCount
}

func (ps *PolicyStats) RulesErrorCount() int {
	return ps.rulesErrorCount
}
