package internal

import (
	"flag"
	"os"
	"testing"
)

func TestPolicyExceptionEnabled(t *testing.T) {
	tests := []struct {
		name      string
		flagValue bool
		expected  bool
	}{
		{
			name:      "policy exceptions enabled",
			flagValue: true,
			expected:  true,
		},
		{
			name:      "policy exceptions disabled",
			flagValue: false,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original value
			originalValue := enablePolicyException
			defer func() {
				enablePolicyException = originalValue
			}()

			// Set test value
			enablePolicyException = tt.flagValue

			// Test the function
			result := PolicyExceptionEnabled()
			if result != tt.expected {
				t.Errorf("PolicyExceptionEnabled() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestExceptionNamespace(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		expected  string
	}{
		{
			name:      "specific namespace",
			namespace: "kyverno",
			expected:  "kyverno",
		},
		{
			name:      "all namespaces",
			namespace: "*",
			expected:  "*",
		},
		{
			name:      "empty namespace",
			namespace: "",
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original value
			originalValue := exceptionNamespace
			defer func() {
				exceptionNamespace = originalValue
			}()

			// Set test value
			exceptionNamespace = tt.namespace

			// Test the function
			result := ExceptionNamespace()
			if result != tt.expected {
				t.Errorf("ExceptionNamespace() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestFlagInitialization(t *testing.T) {
	// Test that flags can be parsed without errors
	// This is important for our CRD availability checking logic

	// Save original command line args
	originalArgs := os.Args
	defer func() {
		os.Args = originalArgs
	}()

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "policy exceptions enabled",
			args: []string{"cmd", "-enablePolicyException=true", "-exceptionNamespace=kyverno"},
		},
		{
			name: "policy exceptions disabled",
			args: []string{"cmd", "-enablePolicyException=false"},
		},
		{
			name: "all namespaces allowed",
			args: []string{"cmd", "-enablePolicyException=true", "-exceptionNamespace=*"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new flag set for this test
			fs := flag.NewFlagSet("test", flag.ContinueOnError)

			// Define the flags we're testing
			var testEnablePolicyException bool
			var testExceptionNamespace string

			fs.BoolVar(&testEnablePolicyException, "enablePolicyException", false, "Enable PolicyException feature.")
			fs.StringVar(&testExceptionNamespace, "exceptionNamespace", "", "Configure the namespace to accept PolicyExceptions.")

			// Parse the test arguments
			err := fs.Parse(tt.args[1:]) // Skip the program name
			if err != nil {
				t.Errorf("Failed to parse flags: %v", err)
				return
			}

			// Verify the flags were parsed correctly
			if tt.name == "policy exceptions enabled" || tt.name == "all namespaces allowed" {
				if !testEnablePolicyException {
					t.Errorf("Expected enablePolicyException to be true")
				}
			} else {
				if testEnablePolicyException {
					t.Errorf("Expected enablePolicyException to be false")
				}
			}
		})
	}
}

// Test the integration of flag parsing with our CRD availability logic
func TestPolicyExceptionFlagIntegration(t *testing.T) {
	// This test simulates the logic we added in main.go
	tests := []struct {
		name                  string
		enablePolicyException bool
		crdAvailable          bool
		expectedFinalState    bool
	}{
		{
			name:                  "feature enabled and CRD available",
			enablePolicyException: true,
			crdAvailable:          true,
			expectedFinalState:    true,
		},
		{
			name:                  "feature enabled but CRD not available",
			enablePolicyException: true,
			crdAvailable:          false,
			expectedFinalState:    false,
		},
		{
			name:                  "feature disabled",
			enablePolicyException: false,
			crdAvailable:          true,
			expectedFinalState:    false,
		},
		{
			name:                  "feature disabled and CRD not available",
			enablePolicyException: false,
			crdAvailable:          false,
			expectedFinalState:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original value
			originalValue := enablePolicyException
			defer func() {
				enablePolicyException = originalValue
			}()

			// Set test value
			enablePolicyException = tt.enablePolicyException

			// Simulate the logic from main.go
			policyExceptionsAvailable := PolicyExceptionEnabled()
			if policyExceptionsAvailable && !tt.crdAvailable {
				// Simulate CRD check failure
				policyExceptionsAvailable = false
			}

			if policyExceptionsAvailable != tt.expectedFinalState {
				t.Errorf("Expected final state %v, got %v", tt.expectedFinalState, policyExceptionsAvailable)
			}
		})
	}
}
