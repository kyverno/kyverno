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

func NewExecutionStats(timestamp time.Time) ExecutionStats {
	return ExecutionStats{
		timestamp: timestamp,
	}
}

func NewExecutionStatsFull(startTime, endTime time.Time) ExecutionStats {
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

func (s *ExecutionStats) Done(timestamp time.Time) {
	s.processingTime = timestamp.Sub(s.timestamp)
}

// PolicyStats stores statistics for the single policy application
type PolicyStats struct {
	// RulesAppliedCount is the count of rules that were applied successfully
	RulesAppliedCount int
	// RulesErrorCount is the count of rules that with execution errors
	RulesErrorCount int
}
