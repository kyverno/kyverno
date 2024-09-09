package policycache

// PolicyType represents types of policies
type PolicyType uint8

// Types of policies
const (
	Mutate PolicyType = 1 << iota
	ValidateEnforce
	ValidateAudit
	ValidateAuditWarn
	Generate
	VerifyImagesMutate
	VerifyImagesValidate
)
