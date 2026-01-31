package metrics

import (
	"testing"
)

func TestPolicyValidationMode(t *testing.T) {
	tests := []struct {
		mode PolicyValidationMode
		want string
	}{
		{Enforce, "enforce"},
		{Audit, "audit"},
	}
	for _, tt := range tests {
		if string(tt.mode) != tt.want {
			t.Errorf("PolicyValidationMode = %v, want %v", tt.mode, tt.want)
		}
	}
}

func TestPolicyType(t *testing.T) {
	tests := []struct {
		ptype PolicyType
		want  string
	}{
		{Cluster, "cluster"},
		{Namespaced, "namespaced"},
	}
	for _, tt := range tests {
		if string(tt.ptype) != tt.want {
			t.Errorf("PolicyType = %v, want %v", tt.ptype, tt.want)
		}
	}
}

func TestPolicyBackgroundMode(t *testing.T) {
	tests := []struct {
		mode PolicyBackgroundMode
		want string
	}{
		{BackgroundTrue, "true"},
		{BackgroundFalse, "false"},
	}
	for _, tt := range tests {
		if string(tt.mode) != tt.want {
			t.Errorf("PolicyBackgroundMode = %v, want %v", tt.mode, tt.want)
		}
	}
}

func TestRuleType(t *testing.T) {
	tests := []struct {
		rtype RuleType
		want  string
	}{
		{Validate, "validate"},
		{Mutate, "mutate"},
		{Generate, "generate"},
		{ImageVerify, "imageVerify"},
		{EmptyRuleType, "-"},
	}
	for _, tt := range tests {
		if string(tt.rtype) != tt.want {
			t.Errorf("RuleType = %v, want %v", tt.rtype, tt.want)
		}
	}
}

func TestRuleResult(t *testing.T) {
	tests := []struct {
		result RuleResult
		want   string
	}{
		{Pass, "pass"},
		{Fail, "fail"},
		{Warn, "warn"},
		{Error, "error"},
		{Skip, "skip"},
	}
	for _, tt := range tests {
		if string(tt.result) != tt.want {
			t.Errorf("RuleResult = %v, want %v", tt.result, tt.want)
		}
	}
}

func TestRuleExecutionCause(t *testing.T) {
	tests := []struct {
		cause RuleExecutionCause
		want  string
	}{
		{AdmissionRequest, "admission_request"},
		{BackgroundScan, "background_scan"},
	}
	for _, tt := range tests {
		if string(tt.cause) != tt.want {
			t.Errorf("RuleExecutionCause = %v, want %v", tt.cause, tt.want)
		}
	}
}

func TestResourceRequestOperation(t *testing.T) {
	tests := []struct {
		op   ResourceRequestOperation
		want string
	}{
		{ResourceCreated, "create"},
		{ResourceUpdated, "update"},
		{ResourceDeleted, "delete"},
		{ResourceConnected, "connect"},
	}
	for _, tt := range tests {
		if string(tt.op) != tt.want {
			t.Errorf("ResourceRequestOperation = %v, want %v", tt.op, tt.want)
		}
	}
}

func TestClientQueryOperation(t *testing.T) {
	tests := []struct {
		op   ClientQueryOperation
		want string
	}{
		{ClientCreate, "create"},
		{ClientGet, "get"},
		{ClientList, "list"},
		{ClientUpdate, "update"},
		{ClientUpdateStatus, "update_status"},
		{ClientDelete, "delete"},
		{ClientDeleteCollection, "delete_collection"},
		{ClientWatch, "watch"},
		{ClientPatch, "patch"},
	}
	for _, tt := range tests {
		if string(tt.op) != tt.want {
			t.Errorf("ClientQueryOperation = %v, want %v", tt.op, tt.want)
		}
	}
}

func TestClientType(t *testing.T) {
	tests := []struct {
		ctype ClientType
		want  string
	}{
		{DynamicClient, "dynamic"},
		{KubeClient, "kubeclient"},
		{KyvernoClient, "kyverno"},
		{MetadataClient, "metadata"},
		{ApiServerClient, "apiserver"},
		{PolicyReportClient, "policyreport"},
	}
	for _, tt := range tests {
		if string(tt.ctype) != tt.want {
			t.Errorf("ClientType = %v, want %v", tt.ctype, tt.want)
		}
	}
}
