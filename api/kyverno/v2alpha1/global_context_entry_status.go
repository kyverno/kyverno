package v2alpha1

import (
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
)

const (
	// PolicyConditionReady means that the globalcontextentry is ready
	GlobalContextEntryConditionReady = kyvernov2beta1.GlobalContextEntryConditionReady
)

const (
	// GlobalContextEntryReasonSucceeded is the reason set when the globalcontextentry is ready
	GlobalContextEntryReasonSucceeded = kyvernov2beta1.GlobalContextEntryReasonSucceeded
	// GlobalContextEntryReasonFailed is the reason set when the globalcontextentry is not ready
	GlobalContextEntryReasonFailed = kyvernov2beta1.GlobalContextEntryReasonFailed
)

type GlobalContextEntryStatus = kyvernov2beta1.GlobalContextEntryStatus
