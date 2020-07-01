package policycache

type PolicyType uint8

const (
	Mutate PolicyType = 1 << iota
	ValidateEnforce
	Generate
)
