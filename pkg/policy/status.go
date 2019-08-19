package policy

import "time"

type PolicyStatus struct {
	// average time required to process the policy rules on a resource
	avgExecutionTime time.Duration
	// Count of rules that were applied succesfully
	rulesAppliedCount int
	// Count of resources for whom update/create api requests were blocked as the resoruce did not satisfy the policy rules
	resourcesBlockedCount int
	// Count of the resource for whom the mutation rules were applied succesfully
	resourcesMutatedCount int
}
