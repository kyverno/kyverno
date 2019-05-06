package utils

type ViolationInfo struct {
	Kind     string
	Resource string
	Policy   string
	Rule     string
	Reason   string
	Message  string
}
