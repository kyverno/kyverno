package policychanges

type PolicyChangeType string

const (
	PolicyCreated PolicyChangeType = "created"
	PolicyUpdated PolicyChangeType = "updated"
	PolicyDeleted PolicyChangeType = "deleted"
)
