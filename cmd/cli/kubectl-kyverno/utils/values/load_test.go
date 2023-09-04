package values

import (
	"os"
	"reflect"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/test/api"
)

func Test_readFile(t *testing.T) {
	mustReadFile := func(path string) []byte {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	tests := []struct {
		name     string
		f        billy.Filesystem
		filepath string
		want     []byte
		wantErr  bool
	}{{
		name:     "empty",
		filepath: "",
		want:     nil,
		wantErr:  true,
	}, {
		name:     "does not exist",
		filepath: "../../testdata/values/doesnotexist",
		want:     nil,
		wantErr:  true,
	}, {
		name:     "bad format",
		filepath: "../../testdata/values/bad-format.yaml",
		want:     mustReadFile("../../testdata/values/bad-format.yaml"),
		wantErr:  false,
	}, {
		name:     "valid",
		filepath: "../../testdata/values/valid.yaml",
		want:     mustReadFile("../../testdata/values/valid.yaml"),
		wantErr:  false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readFile(tt.f, tt.filepath)
			if (err != nil) != tt.wantErr {
				t.Errorf("readFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name     string
		f        billy.Filesystem
		filepath string
		want     *api.Values
		wantErr  bool
	}{{
		name:     "empty",
		filepath: "",
		want:     nil,
		wantErr:  true,
	}, {
		name:     "does not exist",
		filepath: "../../testdata/values/doesnotexist",
		want:     nil,
		wantErr:  true,
	}, {
		name:     "bad format",
		filepath: "../../testdata/values/bad-format.yaml",
		want:     nil,
		wantErr:  true,
	}, {
		name:     "valid",
		filepath: "../../testdata/values/valid.yaml",
		want: &api.Values{
			NamespaceSelectors: []api.NamespaceSelector{{
				Name: "test1",
				Labels: map[string]string{
					"foo.com/managed-state": "managed",
				},
			}},
			Policies: []api.Policy{{
				Name: "limit-configmap-for-sa",
				Resources: []api.Resource{{
					Name: "any-configmap-name-good",
					Values: map[string]interface{}{
						"request.operation": "UPDATE",
					},
				}, {
					Name: "any-configmap-name-bad",
					Values: map[string]interface{}{
						"request.operation": "UPDATE",
					},
				}},
			}},
		},
		wantErr: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Load(tt.f, tt.filepath)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Load() = %v, want %v", got, tt.want)
			}
		})
	}
}
