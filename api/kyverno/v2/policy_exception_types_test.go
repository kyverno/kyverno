package v2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestException_IsFinegrained(t *testing.T) {
	tests := []struct {
		name      string
		exception Exception
		expected  bool
	}{
		{
			name: "empty exception",
			exception: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{"test-rule"},
			},
			expected: false,
		},
		{
			name: "exception with images",
			exception: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{"test-rule"},
				Images: []ImageException{
					{ImageReferences: []string{"test-image"}},
				},
			},
			expected: true,
		},
		{
			name: "exception with values",
			exception: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{"test-rule"},
				Values: []ValueException{
					{Path: "metadata.labels.env", Values: []string{"dev"}},
				},
			},
			expected: true,
		},
		{
			name: "exception with both images and values",
			exception: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{"test-rule"},
				Images: []ImageException{
					{ImageReferences: []string{"test-image"}},
				},
				Values: []ValueException{
					{Path: "metadata.labels.env", Values: []string{"dev"}},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.exception.IsFinegrained()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestException_HasImageExceptions(t *testing.T) {
	tests := []struct {
		name      string
		exception Exception
		expected  bool
	}{
		{
			name: "no image exceptions",
			exception: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{"test-rule"},
			},
			expected: false,
		},
		{
			name: "with image exceptions",
			exception: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{"test-rule"},
				Images: []ImageException{
					{ImageReferences: []string{"test-image"}},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.exception.HasImageExceptions()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestException_HasValueExceptions(t *testing.T) {
	tests := []struct {
		name      string
		exception Exception
		expected  bool
	}{
		{
			name: "no value exceptions",
			exception: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{"test-rule"},
			},
			expected: false,
		},
		{
			name: "with value exceptions",
			exception: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{"test-rule"},
				Values: []ValueException{
					{Path: "metadata.labels.env", Values: []string{"dev"}},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.exception.HasValueExceptions()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestException_GetReportMode(t *testing.T) {
	tests := []struct {
		name      string
		exception Exception
		expected  ExceptionReportMode
	}{
		{
			name: "default report mode",
			exception: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{"test-rule"},
				ReportAs:   nil,
			},
			expected: ExceptionReportSkip,
		},
		{
			name: "warn report mode",
			exception: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{"test-rule"},
				ReportAs:   func() *ExceptionReportMode { mode := ExceptionReportWarn; return &mode }(),
			},
			expected: ExceptionReportWarn,
		},
		{
			name: "pass report mode",
			exception: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{"test-rule"},
				ReportAs:   func() *ExceptionReportMode { mode := ExceptionReportPass; return &mode }(),
			},
			expected: ExceptionReportPass,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.exception.GetReportMode()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestImageException_Validate(t *testing.T) {
	tests := []struct {
		name          string
		imageEx       ImageException
		expectedError bool
	}{
		{
			name: "valid image exception",
			imageEx: ImageException{
				ImageReferences: []string{"nginx:latest", "alpine:*"},
			},
			expectedError: false,
		},
		{
			name: "empty image references",
			imageEx: ImageException{
				ImageReferences: []string{},
			},
			expectedError: true,
		},
		{
			name: "nil image references",
			imageEx: ImageException{
				ImageReferences: nil,
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.imageEx.Validate(field.NewPath("test"))
			hasError := len(errs) > 0
			assert.Equal(t, tt.expectedError, hasError)
		})
	}
}

func TestValueException_Validate(t *testing.T) {
	tests := []struct {
		name          string
		valueEx       ValueException
		expectedError bool
	}{
		{
			name: "valid value exception",
			valueEx: ValueException{
				Path:   "metadata.labels.env",
				Values: []string{"dev", "test"},
			},
			expectedError: false,
		},
		{
			name: "empty path",
			valueEx: ValueException{
				Path:   "",
				Values: []string{"dev"},
			},
			expectedError: true,
		},
		{
			name: "empty values",
			valueEx: ValueException{
				Path:   "metadata.labels.env",
				Values: []string{},
			},
			expectedError: true,
		},
		{
			name: "nil values",
			valueEx: ValueException{
				Path:   "metadata.labels.env",
				Values: nil,
			},
			expectedError: true,
		},
		{
			name: "both empty",
			valueEx: ValueException{
				Path:   "",
				Values: []string{},
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.valueEx.Validate(field.NewPath("test"))
			hasError := len(errs) > 0
			assert.Equal(t, tt.expectedError, hasError)
		})
	}
}

func TestException_Validate_WithFinegrainedExceptions(t *testing.T) {
	tests := []struct {
		name          string
		exception     Exception
		expectedError bool
	}{
		{
			name: "valid exception with images and values",
			exception: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{"test-rule"},
				Images: []ImageException{
					{ImageReferences: []string{"nginx:latest"}},
				},
				Values: []ValueException{
					{Path: "metadata.labels.env", Values: []string{"dev"}},
				},
			},
			expectedError: false,
		},
		{
			name: "exception with invalid image",
			exception: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{"test-rule"},
				Images: []ImageException{
					{ImageReferences: []string{}}, // Invalid
				},
			},
			expectedError: true,
		},
		{
			name: "exception with invalid value",
			exception: Exception{
				PolicyName: "test-policy",
				RuleNames:  []string{"test-rule"},
				Values: []ValueException{
					{Path: "", Values: []string{"dev"}}, // Invalid
				},
			},
			expectedError: true,
		},
		{
			name: "exception with missing policy name",
			exception: Exception{
				PolicyName: "", // Invalid
				RuleNames:  []string{"test-rule"},
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.exception.Validate(field.NewPath("test"))
			hasError := len(errs) > 0
			assert.Equal(t, tt.expectedError, hasError)
		})
	}
}

func TestValueOperatorConstants(t *testing.T) {
	// Test that all ValueOperator constants are defined correctly
	assert.Equal(t, ValueOperator("equals"), ValueOperatorEquals)
	assert.Equal(t, ValueOperator("in"), ValueOperatorIn)
	assert.Equal(t, ValueOperator("startsWith"), ValueOperatorStartsWith)
	assert.Equal(t, ValueOperator("endsWith"), ValueOperatorEndsWith)
	assert.Equal(t, ValueOperator("contains"), ValueOperatorContains)
}

func TestExceptionReportModeConstants(t *testing.T) {
	// Test that all ExceptionReportMode constants are defined correctly
	assert.Equal(t, ExceptionReportMode("skip"), ExceptionReportSkip)
	assert.Equal(t, ExceptionReportMode("warn"), ExceptionReportWarn)
	assert.Equal(t, ExceptionReportMode("pass"), ExceptionReportPass)
}
