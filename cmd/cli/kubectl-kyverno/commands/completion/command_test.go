package completion

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompletionCommand(t *testing.T) {
	cmd := Command()

	// Test command properties
	assert.Equal(t, "completion", cmd.Use[:10])
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
	assert.NotEmpty(t, cmd.Example)
	assert.Equal(t, []string{"bash", "zsh", "fish", "powershell"}, cmd.ValidArgs)
	assert.True(t, cmd.DisableFlagsInUseLine)

	// Test that the command has a valid RunE function
	assert.NotNil(t, cmd.RunE)
}

func TestCompletionCommandExecution(t *testing.T) {
	testCases := []struct {
		name        string
		shell       string
		expectError bool
	}{
		{
			name:        "bash completion",
			shell:       "bash",
			expectError: false,
		},
		{
			name:        "zsh completion",
			shell:       "zsh",
			expectError: false,
		},
		{
			name:        "fish completion",
			shell:       "fish",
			expectError: false,
		},
		{
			name:        "powershell completion",
			shell:       "powershell",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a root command to test completion generation
			rootCmd := &cobra.Command{
				Use: "kyverno",
			}

			// Add the completion command to the root
			completionCmd := Command()
			rootCmd.AddCommand(completionCmd)

			// Execute the completion command
			rootCmd.SetArgs([]string{"completion", tc.shell})
			err := rootCmd.Execute()

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCompletionCommandArgs(t *testing.T) {
	testCases := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "valid bash arg",
			args:        []string{"bash"},
			expectError: false,
		},
		{
			name:        "valid zsh arg",
			args:        []string{"zsh"},
			expectError: false,
		},
		{
			name:        "valid fish arg",
			args:        []string{"fish"},
			expectError: false,
		},
		{
			name:        "valid powershell arg",
			args:        []string{"powershell"},
			expectError: false,
		},
		{
			name:        "invalid shell arg",
			args:        []string{"invalid"},
			expectError: true,
		},
		{
			name:        "no args",
			args:        []string{},
			expectError: true,
		},
		{
			name:        "too many args",
			args:        []string{"bash", "extra"},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := Command()
			cmd.SetArgs(tc.args)

			// Redirect output to avoid cluttering test output
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)

			err := cmd.Execute()
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCompletionDocValues(t *testing.T) {
	// Test that doc.go constants are properly defined
	assert.NotEmpty(t, websiteUrl, "websiteUrl should not be empty")
	assert.Contains(t, websiteUrl, "kyverno.io", "websiteUrl should contain kyverno.io domain")
	assert.Contains(t, websiteUrl, "shell-autocompletion", "websiteUrl should contain shell-autocompletion")

	assert.NotEmpty(t, description, "description should not be empty")
	assert.Greater(t, len(description), 5, "description should have multiple lines")

	// Check that description contains key terms
	descText := strings.Join(description, " ")
	assert.Contains(t, descText, "autocompletion", "description should mention autocompletion")
	assert.Contains(t, descText, "shell", "description should mention shell")
	assert.Contains(t, descText, "tab completion", "description should mention tab completion")

	assert.NotEmpty(t, examples, "examples should not be empty")
	assert.Greater(t, len(examples), 5, "examples should have multiple examples")

	// Check that examples cover all supported shells
	exampleText := ""
	for _, example := range examples {
		for _, line := range example {
			exampleText += line + " "
		}
	}

	assert.Contains(t, exampleText, "bash", "examples should include bash")
	assert.Contains(t, exampleText, "zsh", "examples should include zsh")
	assert.Contains(t, exampleText, "fish", "examples should include fish")
	assert.Contains(t, exampleText, "powershell", "examples should include powershell")
}

func TestCompletionCommandIntegration(t *testing.T) {
	// Test the completion command as part of a larger command tree
	rootCmd := &cobra.Command{
		Use: "kyverno",
	}

	// Add some dummy subcommands to test completion
	applyCmd := &cobra.Command{
		Use: "apply",
	}
	testCmd := &cobra.Command{
		Use: "test",
	}

	rootCmd.AddCommand(applyCmd)
	rootCmd.AddCommand(testCmd)
	rootCmd.AddCommand(Command())

	// Test that completion command is properly integrated
	completionCmd, _, err := rootCmd.Find([]string{"completion"})
	require.NoError(t, err)
	assert.Equal(t, "completion", completionCmd.Name())

	// Test executing completion for bash to ensure it works in context
	rootCmd.SetArgs([]string{"completion", "bash"})

	err = rootCmd.Execute()
	assert.NoError(t, err)
}

func TestCompletionRunEFunction(t *testing.T) {
	// Test the RunE function directly
	cmd := Command()

	testCases := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "bash completion function",
			args:        []string{"bash"},
			expectError: false,
		},
		{
			name:        "zsh completion function",
			args:        []string{"zsh"},
			expectError: false,
		},
		{
			name:        "fish completion function",
			args:        []string{"fish"},
			expectError: false,
		},
		{
			name:        "powershell completion function",
			args:        []string{"powershell"},
			expectError: false,
		},
		{
			name:        "unsupported shell (for coverage)",
			args:        []string{"unsupported"},
			expectError: false, // This will return nil (no error)
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a root command to ensure proper context
			rootCmd := &cobra.Command{
				Use: "kyverno",
			}
			rootCmd.AddCommand(cmd)

			// Call the RunE function directly (bypassing Args validation)
			err := cmd.RunE(cmd, tc.args)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
