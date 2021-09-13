package policychanges

import (
	"github.com/kyverno/kyverno/pkg/metrics"
)

type PolicyChangeType string

const (
	PolicyCreated PolicyChangeType = "created"
	PolicyUpdated PolicyChangeType = "updated"
	PolicyDeleted PolicyChangeType = "deleted"
)

type PromMetrics metrics.PromMetrics

type PromConfig metrics.PromConfig
