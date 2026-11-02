package yaml

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitDocuments(t *testing.T) {
	type args struct {
		yamlBytes []byte
	}
	tests := []struct {
		name          string
		args          args
		wantDocuments []string
		wantErr       bool
	}{{
		name: "nil",
		args: args{
			nil,
		},
		wantDocuments: nil,
		wantErr:       false,
	}, {
		name: "empty string",
		args: args{
			[]byte(""),
		},
		wantDocuments: nil,
		wantErr:       false,
	}, {
		name: "single doc",
		args: args{
			[]byte("enabled: true"),
		},
		wantDocuments: []string{
			"enabled: true\n",
		},
		wantErr: false,
	}, {
		name: "two docs",
		args: args{
			[]byte("enabled: true\n---\ndisabled: false"),
		},
		wantDocuments: []string{
			"enabled: true\n",
			"disabled: false\n",
		},
		wantErr: false,
	},
		{
			name: "empty doc",
			args: args{
				[]byte("enabled: true\n---\n---\ndisabled: false"),
			},
			wantDocuments: []string{
				"enabled: true\n",
				"---\ndisabled: false\n",
			},
			wantErr: false,
		},
		{
			name: "only separators",
			args: args{
				[]byte("---\n---\n"),
			},
			wantDocuments: []string{
				"---\n",
			},
			wantErr: false,
		},
		{
			name: "only separators",
			args: args{
				[]byte("---\n\n\n---\n"),
			},
			wantDocuments: []string{
				"---\n\n\n",
			},
			wantErr: false,
		},
		{
			// A "---" line followed by anything other than a comment is an
			// invalid document separator. Previously this was swallowed
			// silently: err was nil and every document was dropped,
			// including the valid ones read before the bad separator.
			name: "invalid separator with trailing text",
			args: args{
				[]byte("enabled: true\n--- oops\ndisabled: false"),
			},
			wantDocuments: nil,
			wantErr:       true,
		},
		{
			// "second: true" is lost even with the fix below: k8s.io's own
			// YAMLReader.Read() discovers the bad separator while it is
			// still accumulating this document's lines into its internal
			// buffer, and returns the syntax error without ever handing
			// that buffer back, in either the fixed or the original code.
			// Only documents a *prior* Read() call already finished (like
			// "first: true" here) can survive.
			name: "invalid separator after multiple valid documents",
			args: args{
				[]byte("first: true\n---\nsecond: true\n--- oops\nthird: true"),
			},
			wantDocuments: []string{
				"first: true\n",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDocuments, err := SplitDocuments(tt.args.yamlBytes)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, len(tt.wantDocuments), len(gotDocuments))
			for i := range gotDocuments {
				assert.Equal(t, tt.wantDocuments[i], string(gotDocuments[i]))
			}
		})
	}
}
