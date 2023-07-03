package api

import (
	"reflect"
	"testing"
	"time"
)

func TestNewExecutionStats(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name      string
		startTime time.Time
		endTime   time.Time
		want      ExecutionStats
	}{{
		startTime: now,
		endTime:   now,
		want:      ExecutionStats{0, now},
	}, {
		startTime: now,
		endTime:   now.Add(time.Hour),
		want:      ExecutionStats{time.Hour, now},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewExecutionStats(tt.startTime, tt.endTime); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewExecutionStats() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExecutionStats_Time(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name      string
		startTime time.Time
		endTime   time.Time
		want      time.Time
	}{{
		startTime: now,
		endTime:   now,
		want:      now,
	}, {
		startTime: now,
		endTime:   now.Add(time.Hour),
		want:      now,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewExecutionStats(tt.startTime, tt.endTime)
			if got := s.Time(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExecutionStats.Time() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExecutionStats_Timestamp(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name      string
		startTime time.Time
		endTime   time.Time
		want      int64
	}{{
		startTime: now,
		endTime:   now,
		want:      now.Unix(),
	}, {
		startTime: now,
		endTime:   now.Add(time.Hour),
		want:      now.Unix(),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewExecutionStats(tt.startTime, tt.endTime)
			if got := s.Timestamp(); got != tt.want {
				t.Errorf("ExecutionStats.Timestamp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExecutionStats_ProcessingTime(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name      string
		startTime time.Time
		endTime   time.Time
		want      time.Duration
	}{{
		startTime: now,
		endTime:   now,
		want:      0,
	}, {
		startTime: now,
		endTime:   now.Add(time.Hour),
		want:      time.Hour,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewExecutionStats(tt.startTime, tt.endTime)
			if got := s.ProcessingTime(); got != tt.want {
				t.Errorf("ExecutionStats.ProcessingTime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPolicyStats_RulesAppliedCount(t *testing.T) {
	tests := []struct {
		name              string
		rulesAppliedCount int
		rulesErrorCount   int
		want              int
	}{{}, {
		rulesAppliedCount: 10,
		rulesErrorCount:   20,
		want:              10,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := &PolicyStats{
				rulesAppliedCount: tt.rulesAppliedCount,
				rulesErrorCount:   tt.rulesErrorCount,
			}
			if got := ps.RulesAppliedCount(); got != tt.want {
				t.Errorf("PolicyStats.RulesAppliedCount() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPolicyStats_RulesErrorCount(t *testing.T) {
	tests := []struct {
		name              string
		rulesAppliedCount int
		rulesErrorCount   int
		want              int
	}{{}, {
		rulesAppliedCount: 10,
		rulesErrorCount:   20,
		want:              20,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := &PolicyStats{
				rulesAppliedCount: tt.rulesAppliedCount,
				rulesErrorCount:   tt.rulesErrorCount,
			}
			if got := ps.RulesErrorCount(); got != tt.want {
				t.Errorf("PolicyStats.RulesErrorCount() = %v, want %v", got, tt.want)
			}
		})
	}
}
