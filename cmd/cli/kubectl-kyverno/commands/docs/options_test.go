package docs

import (
	"os"
	"path/filepath"
	"testing"
)

func Test_postProcessDocs(t *testing.T) {
	tests := []struct {
		name          string
		files         map[string]string
		expectedFiles map[string]string
		expectError   bool
	}{
		{
			name: "process flags with type hints and varying spaces",
			files: map[string]string{
				"test.md": "### Options\n```\n  -h, --help   help text\n  -v, --v Level   log level\n  --no-desc\n```",
			},
			expectedFiles: map[string]string{
				"test.md": "### Options\n* **<a id=\"help\"></a>-h, --help**: help text\n* **<a id=\"v\"></a>-v, --v Level**: log level\n* **<a id=\"no-desc\"></a>--no-desc**: \n",
			},
		},
		{
			name: "file without Options section",
			files: map[string]string{
				"no_opts.md": "## Title\nJust some text.\n",
			},
			expectedFiles: map[string]string{
				"no_opts.md": "## Title\nJust some text.\n",
			},
		},
		{
			name: "non-md files stay unchanged",
			files: map[string]string{
				"main.go":  "### Options\n```\n  -h, --help  help\n```\n",
				"test.txt": "some text",
			},
			expectedFiles: map[string]string{
				"main.go":  "### Options\n```\n  -h, --help  help\n```\n",
				"test.txt": "some text",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			for name, content := range tt.files {
				err := os.WriteFile(filepath.Join(tempDir, name), []byte(content), 0644)
				if err != nil {
					t.Fatalf("failed to write test file: %v", err)
				}
			}

			err := postProcessDocs(tempDir)

			if (err != nil) != tt.expectError {
				t.Errorf("expected error: %v, got: %v", tt.expectError, err)
			}

			for name, expectedContent := range tt.expectedFiles {
				content, err := os.ReadFile(filepath.Join(tempDir, name))
				if err != nil {
					t.Fatalf("failed to read file %s: %v", name, err)
				}
				if string(content) != expectedContent {
					t.Errorf("file %s content mismatch.\nExpected:\n%s\nGot:\n%s", name, expectedContent, string(content))
				}
			}
		})
	}
}
