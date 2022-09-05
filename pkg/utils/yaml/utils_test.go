package yaml

import (
	"testing"
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
			if (err != nil) != tt.wantErr {
				t.Errorf("SplitDocuments() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(gotDocuments) != len(tt.wantDocuments) {
				t.Errorf("SplitDocuments() docs count = %v, want %v", len(gotDocuments), len(tt.wantDocuments))
				return
			}
			for i := range gotDocuments {
				if string(gotDocuments[i]) != tt.wantDocuments[i] {
					t.Errorf("SplitDocuments() doc %v = %v, want %v", i, string(gotDocuments[i]), tt.wantDocuments[i])
				}
			}
		})
	}
}
