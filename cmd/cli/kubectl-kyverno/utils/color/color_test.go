package color

import (
	"testing"
)

func TestInitColors(t *testing.T) {
	tests := []struct {
		name    string
		noColor bool
	}{{
		noColor: true,
	}, {
		noColor: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			InitColors(tt.noColor)
		})
	}
}

func TestPolicy(t *testing.T) {
	InitColors(true)
	tests := []struct {
		name            string
		policyNamespace string
		policyName      string
		want            string
	}{{
		policyNamespace: "",
		policyName:      "policy",
		want:            "policy",
	}, {
		policyNamespace: "namespace",
		policyName:      "policy",
		want:            "namespace/policy",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Policy(tt.policyNamespace, tt.policyName); got != tt.want {
				t.Errorf("Policy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRule(t *testing.T) {
	InitColors(true)
	tests := []struct {
		name     string
		ruleName string
		want     string
	}{{
		ruleName: "rule",
		want:     "rule",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Rule(tt.ruleName); got != tt.want {
				t.Errorf("Rule() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResource(t *testing.T) {
	InitColors(true)
	tests := []struct {
		name         string
		kind         string
		namespace    string
		resourceName string
		want         string
	}{{
		kind:         "Namespace",
		namespace:    "",
		resourceName: "resource",
		want:         "Namespace/resource",
	}, {
		kind:         "Pod",
		namespace:    "namespace",
		resourceName: "resource",
		want:         "namespace/Pod/resource",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Resource(tt.kind, tt.namespace, tt.resourceName); got != tt.want {
				t.Errorf("Resource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNotFound(t *testing.T) {
	InitColors(true)
	tests := []struct {
		name string
		want string
	}{{
		want: "Not found",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NotFound(); got != tt.want {
				t.Errorf("NotFound() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResultPass(t *testing.T) {
	InitColors(true)
	tests := []struct {
		name string
		want string
	}{{
		want: "Pass",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResultPass(); got != tt.want {
				t.Errorf("ResultPass() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResultFail(t *testing.T) {
	InitColors(true)
	tests := []struct {
		name string
		want string
	}{{
		want: "Fail",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResultFail(); got != tt.want {
				t.Errorf("ResultFail() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResultWarn(t *testing.T) {
	InitColors(true)
	tests := []struct {
		name string
		want string
	}{{
		want: "Warn",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResultWarn(); got != tt.want {
				t.Errorf("ResultWarn() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResultError(t *testing.T) {
	InitColors(true)
	tests := []struct {
		name string
		want string
	}{{
		want: "Error",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResultError(); got != tt.want {
				t.Errorf("ResultError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResultSkip(t *testing.T) {
	InitColors(true)
	tests := []struct {
		name string
		want string
	}{{
		want: "Skip",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResultSkip(); got != tt.want {
				t.Errorf("ResultSkip() = %v, want %v", got, tt.want)
			}
		})
	}
}
