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
		// TODO those tests should fail IMHO
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDocuments, err := SplitDocuments(tt.args.yamlBytes)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.wantDocuments), len(gotDocuments))
				for i := range gotDocuments {
					assert.Equal(t, tt.wantDocuments[i], string(gotDocuments[i]))
				}
			}
		})
	}
}
