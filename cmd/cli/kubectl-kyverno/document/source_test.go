package document

import (
	"fmt"
	"reflect"
	"testing"

	gitutils "github.com/kyverno/kyverno/pkg/utils/git"
)

func TestNewSource(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		want    Source
		wantErr bool
	}{
		{
			name:    "local folder",
			src:     "../testdata/tests/test-1",
			want:    fileSystem("../testdata/tests/test-1"),
			wantErr: false,
		},
		{
			name:    "git",
			src:     "https://github.com/kyverno/policies",
			want:    fileSystem("../testdata/tests/test-1"),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewSource(tt.src)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewSource() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			docs, err := got.GetDocuments(gitutils.IsYaml)
			fmt.Println(docs)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewSource() = %v, want %v", got, tt.want)
			}
		})
	}
}
