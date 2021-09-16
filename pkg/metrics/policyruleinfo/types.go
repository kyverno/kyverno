package policyruleinfo

import (
	"github.com/kyverno/kyverno/pkg/metrics"
)

type PolicyRuleInfoMetricChangeType string

const (
	PolicyRuleCreated PolicyRuleInfoMetricChangeType = "created"
	PolicyRuleDeleted PolicyRuleInfoMetricChangeType = "deleted"
)

type PromMetrics metrics.PromMetrics

type PromConfig metrics.PromConfig
