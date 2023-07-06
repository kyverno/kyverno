package kyverno

type RequestType string

const (
	Mutate   RequestType = "mutate"
	Generate RequestType = "generate"
)
