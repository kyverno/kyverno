package document

import (
	"reflect"
	"testing"
)

func Test_fileDocument_Content(t *testing.T) {
	tests := []struct {
		name    string
		d       fileDocument
		want    []byte
		wantErr bool
	}{
		{
			name:    "empty",
			d:       fileDocument(""),
			want:    nil,
			wantErr: true,
		},
		{
			name:    "directory",
			d:       fileDocument("."),
			want:    nil,
			wantErr: true,
		},
		{
			name:    "existing",
			d:       fileDocument("../testdata/tests/test-1/not-kyverno-test.yaml"),
			want:    nil,
			wantErr: true,
		},
		{
			name: "existing",
			d:    fileDocument("../testdata/tests/test-1/kyverno-test.yaml"),
			want: []byte(`name: test-registry
policies:
- image-example.yaml
resources:
- resources.yaml
results:
- kind: Pod
  policy: images
  resources:
  - test-pod-with-non-root-user-image
  result: pass
  rule: only-allow-trusted-images
- kind: Pod
  policy: images
  resources:
  - test-pod-with-trusted-registry
  result: pass
  rule: only-allow-trusted-images
`),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.d.Content()
			if (err != nil) != tt.wantErr {
				t.Errorf("fileDocument.Content() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("fileDocument.Content() = %v, want %v", got, tt.want)
			}
		})
	}
}
