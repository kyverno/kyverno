package v2

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestException_HasImageExceptions(t *testing.T) {
	tests := []struct {
		name     string
		exc      Exception
		expected bool
	}{
		{
			name: "has image exceptions",
			exc: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{"rule1"},
				Images: []ImageException{
					{
						ImageReferences: []string{"nginx:*"},
					},
				},
			},
			expected: true,
		},
		{
			name: "no image exceptions",
			exc: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{"rule1"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.exc.HasImageExceptions(); got != tt.expected {
				t.Errorf("HasImageExceptions() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestException_HasValueExceptions(t *testing.T) {
	tests := []struct {
		name     string
		exc      Exception
		expected bool
	}{
		{
			name: "has value exceptions",
			exc: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{"rule1"},
				Values: []ValueException{
					{
						Path:   "spec.containers[*].securityContext.runAsUser",
						Values: []string{"1000", "2000"},
					},
				},
			},
			expected: true,
		},
		{
			name: "no value exceptions",
			exc: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{"rule1"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.exc.HasValueExceptions(); got != tt.expected {
				t.Errorf("HasValueExceptions() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestException_IsFinegrained(t *testing.T) {
	tests := []struct {
		name     string
		exc      Exception
		expected bool
	}{
		{
			name: "has image exceptions",
			exc: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{"rule1"},
				Images: []ImageException{
					{
						ImageReferences: []string{"nginx:*"},
					},
				},
			},
			expected: true,
		},
		{
			name: "has value exceptions",
			exc: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{"rule1"},
				Values: []ValueException{
					{
						Path:   "spec.containers[*].securityContext.runAsUser",
						Values: []string{"1000"},
					},
				},
			},
			expected: true,
		},
		{
			name: "has both exceptions",
			exc: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{"rule1"},
				Images: []ImageException{
					{
						ImageReferences: []string{"nginx:*"},
					},
				},
				Values: []ValueException{
					{
						Path:   "spec.containers[*].securityContext.runAsUser",
						Values: []string{"1000"},
					},
				},
			},
			expected: true,
		},
		{
			name: "no fine-grained exceptions",
			exc: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{"rule1"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.exc.IsFinegrained(); got != tt.expected {
				t.Errorf("IsFinegrained() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestException_GetReportMode(t *testing.T) {
	skip := ExceptionReportSkip
	warn := ExceptionReportWarn
	pass := ExceptionReportPass

	tests := []struct {
		name     string
		exc      Exception
		expected ExceptionReportMode
	}{
		{
			name: "default report mode",
			exc: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{"rule1"},
			},
			expected: ExceptionReportSkip,
		},
		{
			name: "explicit skip mode",
			exc: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{"rule1"},
				ReportAs:   &skip,
			},
			expected: ExceptionReportSkip,
		},
		{
			name: "warn mode",
			exc: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{"rule1"},
				ReportAs:   &warn,
			},
			expected: ExceptionReportWarn,
		},
		{
			name: "pass mode",
			exc: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{"rule1"},
				ReportAs:   &pass,
			},
			expected: ExceptionReportPass,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.exc.GetReportMode(); got != tt.expected {
				t.Errorf("GetReportMode() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestImageException_Validate(t *testing.T) {
	tests := []struct {
		name        string
		imgExc      ImageException
		expectError bool
	}{
		{
			name: "valid image exception",
			imgExc: ImageException{
				ImageReferences: []string{"nginx:*", "alpine:latest"},
			},
			expectError: false,
		},
		{
			name: "empty image references",
			imgExc: ImageException{
				ImageReferences: []string{},
			},
			expectError: true,
		},
		{
			name: "nil image references",
			imgExc: ImageException{
				ImageReferences: nil,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.imgExc.Validate(field.NewPath("test"))
			hasErrors := len(errs) > 0

			if hasErrors != tt.expectError {
				t.Errorf("Validate() hasErrors = %v, expectError %v, errors: %v", hasErrors, tt.expectError, errs)
			}
		})
	}
}

func TestValueException_Validate(t *testing.T) {
	tests := []struct {
		name        string
		valExc      ValueException
		expectError bool
	}{
		{
			name: "valid value exception",
			valExc: ValueException{
				Path:   "spec.containers[*].securityContext.runAsUser",
				Values: []string{"1000", "2000"},
			},
			expectError: false,
		},
		{
			name: "empty path",
			valExc: ValueException{
				Path:   "",
				Values: []string{"1000"},
			},
			expectError: true,
		},
		{
			name: "empty values",
			valExc: ValueException{
				Path:   "spec.containers[*].securityContext.runAsUser",
				Values: []string{},
			},
			expectError: true,
		},
		{
			name: "nil values",
			valExc: ValueException{
				Path:   "spec.containers[*].securityContext.runAsUser",
				Values: nil,
			},
			expectError: true,
		},
		{
			name: "both empty",
			valExc: ValueException{
				Path:   "",
				Values: []string{},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.valExc.Validate(field.NewPath("test"))
			hasErrors := len(errs) > 0

			if hasErrors != tt.expectError {
				t.Errorf("Validate() hasErrors = %v, expectError %v, errors: %v", hasErrors, tt.expectError, errs)
			}
		})
	}
}

func TestException_Validate(t *testing.T) {
	tests := []struct {
		name        string
		exc         Exception
		expectError bool
	}{
		{
			name: "valid exception",
			exc: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{"rule1"},
				Images: []ImageException{
					{
						ImageReferences: []string{"nginx:*"},
					},
				},
				Values: []ValueException{
					{
						Path:   "spec.containers[*].securityContext.runAsUser",
						Values: []string{"1000"},
					},
				},
			},
			expectError: false,
		},
		{
			name: "empty policy name",
			exc: Exception{
				PolicyName: "",
				RuleNames:  []string{"rule1"},
			},
			expectError: true,
		},
		{
			name: "invalid image exception",
			exc: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{"rule1"},
				Images: []ImageException{
					{
						ImageReferences: []string{}, // Invalid
					},
				},
			},
			expectError: true,
		},
		{
			name: "invalid value exception",
			exc: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{"rule1"},
				Values: []ValueException{
					{
						Path:   "", // Invalid
						Values: []string{"1000"},
					},
				},
			},
			expectError: true,
		},
		{
			name: "multiple validation errors",
			exc: Exception{
				PolicyName: "", // Invalid
				RuleNames:  []string{"rule1"},
				Images: []ImageException{
					{
						ImageReferences: []string{}, // Invalid
					},
				},
				Values: []ValueException{
					{
						Path:   "", // Invalid
						Values: []string{"1000"},
					},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.exc.Validate(field.NewPath("test"))
			hasErrors := len(errs) > 0

			if hasErrors != tt.expectError {
				t.Errorf("Validate() hasErrors = %v, expectError %v, errors: %v", hasErrors, tt.expectError, errs)
			}
		})
	}
}

func TestException_Contains(t *testing.T) {
	tests := []struct {
		name     string
		exc      Exception
		policy   string
		rule     string
		expected bool
	}{
		{
			name: "exact policy and rule match",
			exc: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{"rule1", "rule2"},
			},
			policy:   "test-policy",
			rule:     "rule1",
			expected: true,
		},
		{
			name: "policy match, rule not found",
			exc: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{"rule1", "rule2"},
			},
			policy:   "test-policy",
			rule:     "rule3",
			expected: false,
		},
		{
			name: "policy mismatch",
			exc: Exception{
				PolicyName: "other-policy",
				RuleNames:  []string{"rule1", "rule2"},
			},
			policy:   "test-policy",
			rule:     "rule1",
			expected: false,
		},
		{
			name: "empty rule names",
			exc: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{},
			},
			policy:   "test-policy",
			rule:     "rule1",
			expected: false,
		},
		{
			name: "namespaced policy",
			exc: Exception{
				PolicyName: "namespace/test-policy",
				RuleNames:  []string{"rule1"},
			},
			policy:   "namespace/test-policy",
			rule:     "rule1",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.exc.Contains(tt.policy, tt.rule); got != tt.expected {
				t.Errorf("Contains() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestValueOperatorConstants(t *testing.T) {
	// Test that all value operator constants are properly defined
	operators := []ValueOperator{
		ValueOperatorEquals,
		ValueOperatorIn,
		ValueOperatorStartsWith,
		ValueOperatorEndsWith,
		ValueOperatorContains,
	}

	expectedValues := []string{
		"equals",
		"in",
		"startsWith",
		"endsWith",
		"contains",
	}

	for i, op := range operators {
		if string(op) != expectedValues[i] {
			t.Errorf("ValueOperator constant %d = %v, want %v", i, string(op), expectedValues[i])
		}
	}
}

func TestExceptionReportModeConstants(t *testing.T) {
	// Test that all exception report mode constants are properly defined
	modes := []ExceptionReportMode{
		ExceptionReportSkip,
		ExceptionReportWarn,
		ExceptionReportPass,
	}

	expectedValues := []string{
		"skip",
		"warn",
		"pass",
	}

	for i, mode := range modes {
		if string(mode) != expectedValues[i] {
			t.Errorf("ExceptionReportMode constant %d = %v, want %v", i, string(mode), expectedValues[i])
		}
	}
}
