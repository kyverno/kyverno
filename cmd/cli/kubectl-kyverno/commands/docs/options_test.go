package docs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func createTestCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "kyverno"}
	cmd.Flags().StringP("help", "h", "", "help text")
	cmd.Flags().StringP("v", "v", "", "log level")
	cmd.Flags().Bool("no-desc", false, "")
	cmd.Flags().StringP("type", "t", "", "type description")
	cmd.Flags().StringP("wildcard", "w", "", "wildcard desc")
	cmd.Flags().BoolP("force", "f", false, "force action")
	cmd.Flags().String("example", "", "value")
	return cmd
}

func Test_postProcessDocs(t *testing.T) {
	tests := []struct {
		name          string
		files         map[string]string
		expectedFiles map[string]string
		expectError   bool
		invalidDir    bool
	}{
		{
			name: "process flags with type hints and varying spaces",
			files: map[string]string{
				"kyverno.md": "### Options\n```\n  -h, --help   help text\n  -v, --v Level   log level\n  --no-desc\n```\n",
			},
			expectedFiles: map[string]string{
				"kyverno.md": "### Options\n* **<a id=\"help\"></a>-h, --help**: help text\n* **<a id=\"v\"></a>-v, --v Level**: log level\n* **<a id=\"no-desc\"></a>--no-desc**: \n",
			},
		},
		{
			name: "file without Options section",
			files: map[string]string{
				"kyverno_no_opts.md": "## Title\nJust some text.\n",
			},
			expectedFiles: map[string]string{
				"kyverno_no_opts.md": "## Title\nJust some text.\n",
			},
		},
		{
			name: "non-md files stay unchanged",
			files: map[string]string{
				"main.go":  "### Options\n```\n  -h, --help  help\n```\n",
				"test.txt": "some text\n",
			},
			expectedFiles: map[string]string{
				"main.go":  "### Options\n```\n  -h, --help  help\n```\n",
				"test.txt": "some text\n",
			},
		},
		{
			name: "escaping markdown and html characters with unmatched anchor fallback",
			files: map[string]string{
				"kyverno.md": "### Options\n```\n  -t, --type <string>   type description\n  -w, --wildcard *   wildcard desc\n  unmatched plain text line\n```\n",
			},
			expectedFiles: map[string]string{
				"kyverno.md": "### Options\n* **<a id=\"type\"></a>-t, --type &lt;string&gt;**: type description\n* **<a id=\"wildcard\"></a>-w, --wildcard \\***: wildcard desc\n  unmatched plain text line\n",
			},
		},
		{
			name: "unrelated markdown file with Options stays unchanged",
			files: map[string]string{
				"unrelated_doc.md": "### Options\n```\n  -h, --help   help text\n```\n",
			},
			expectedFiles: map[string]string{
				"unrelated_doc.md": "### Options\n```\n  -h, --help   help text\n```\n",
			},
		},
		{
			name: "preserve flag-like and heading-like lines inside inner code fences",
			files: map[string]string{
				"kyverno.md": "### Options\n```\n  -h, --help   help text\n```\nSome text with an inner block:\n```yaml\n  --example  value\n### Example\n```\n### SEE ALSO\n",
			},
			expectedFiles: map[string]string{
				"kyverno.md": "### Options\n* **<a id=\"help\"></a>-h, --help**: help text\nSome text with an inner block:\n```yaml\n  --example  value\n### Example\n```\n\n### SEE ALSO\n",
			},
		},
		{
			name: "mismatched inner code fences do not toggle state early",
			files: map[string]string{
				"kyverno.md": "### Options\n```\n  -h, --help   help text\n```\n```yaml\n~~~\n  --example  value\n### Example\n```\n### SEE ALSO\n",
			},
			expectedFiles: map[string]string{
				"kyverno.md": "### Options\n* **<a id=\"help\"></a>-h, --help**: help text\n```yaml\n~~~\n  --example  value\n### Example\n```\n\n### SEE ALSO\n",
			},
		},
		{
			name: "mismatched inner code fences with trailing text do not close",
			files: map[string]string{
				"kyverno.md": "### Options\n```\n  -h, --help   help text\n```\n```yaml\n```not-a-close\n  --example  value\n### Example\n```\n### SEE ALSO\n",
			},
			expectedFiles: map[string]string{
				"kyverno.md": "### Options\n* **<a id=\"help\"></a>-h, --help**: help text\n```yaml\n```not-a-close\n  --example  value\n### Example\n```\n\n### SEE ALSO\n",
			},
		},
		{
			name: "global heading spacing applied to all section transitions",
			files: map[string]string{
				"kyverno.md": "### Options\n```\n  -f, --force   force action\n```\n### SEE ALSO\nSome text\n### HISTORY\n",
			},
			expectedFiles: map[string]string{
				"kyverno.md": "### Options\n* **<a id=\"force\"></a>-f, --force**: force action\n\n### SEE ALSO\nSome text\n\n### HISTORY\n",
			},
		},
		{
			name: "process both options and inherited options",
			files: map[string]string{
				"kyverno.md": "### Options\n```\n  -f, --force   force action\n```\n### Options inherited from parent commands\n```\n  -v, --v Level   log level\n```\n",
			},
			expectedFiles: map[string]string{
				"kyverno.md": "### Options\n* **<a id=\"force\"></a>-f, --force**: force action\n\n### Options inherited from parent commands\n* **<a id=\"v\"></a>-v, --v Level**: log level\n",
			},
		},
		{
			name:        "nonexistent directory returns error",
			expectError: true,
			invalidDir:  true,
		},
	}

	testCmd := createTestCmd()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			for name, content := range tt.files {
				err := os.WriteFile(filepath.Join(tempDir, name), []byte(content), 0644)
				if err != nil {
					t.Fatalf("failed to write test file: %v", err)
				}
			}

			targetDir := tempDir
			if tt.invalidDir {
				targetDir = filepath.Join(tempDir, "does-not-exist")
			}

			err := postProcessDocs(targetDir, testCmd)

			if (err != nil) != tt.expectError {
				t.Errorf("expected error: %v, got: %v (err: %v)", tt.expectError, err != nil, err)
			}

			if !tt.expectError {
				for name, expectedContent := range tt.expectedFiles {
					content, err := os.ReadFile(filepath.Join(tempDir, name))
					if err != nil {
						t.Fatalf("failed to read file %s: %v", name, err)
					}
					if string(content) != expectedContent {
						t.Errorf("file %s content mismatch.\nExpected:\n%q\nGot:\n%q", name, expectedContent, string(content))
					}
				}
			}
		})
	}
}

func TestIdempotency_PostProcessDocs(t *testing.T) {
	tempDir := t.TempDir()
	fileName := "kyverno.md"

	initialContent := "### Options\n* **<a id=\"help\"></a>-h, --help**: help text with example:\n```yaml\nvalue\n```\n\n### SEE ALSO\n"

	err := os.WriteFile(filepath.Join(tempDir, fileName), []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	testCmd := createTestCmd()

	if err := postProcessDocs(tempDir, testCmd); err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	content1, _ := os.ReadFile(filepath.Join(tempDir, fileName))

	if err := postProcessDocs(tempDir, testCmd); err != nil {
		t.Fatalf("second run failed: %v", err)
	}
	content2, _ := os.ReadFile(filepath.Join(tempDir, fileName))

	if string(content1) != string(content2) {
		t.Errorf("idempotency violated.\nFirst run:\n%q\nSecond run:\n%q", string(content1), string(content2))
	}
}
