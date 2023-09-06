package values

import (
	"os"
	"reflect"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	valuesapi "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/values"
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
		filepath: "../_testdata/values/doesnotexist",
		want:     nil,
		wantErr:  true,
	}, {
		name:     "bad format",
		filepath: "../_testdata/values/bad-format.yaml",
		want:     mustReadFile("../_testdata/values/bad-format.yaml"),
		wantErr:  false,
	}, {
		name:     "valid",
		filepath: "../_testdata/values/valid.yaml",
		want:     mustReadFile("../_testdata/values/valid.yaml"),
		wantErr:  false,
	}, {
		name:     "empty (billy)",
		f:        memfs.New(),
		filepath: "",
		want:     nil,
		wantErr:  true,
	}, {
		name: "valid (billy)",
		f: func() billy.Filesystem {
			f := memfs.New()
			file, err := f.Create("valid.yaml")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := file.Write([]byte("foo: bar")); err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			return f
		}(),
		filepath: "valid.yaml",
		want:     []byte("foo: bar"),
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
		want     *valuesapi.Values
		wantErr  bool
	}{{
		name:     "empty",
		filepath: "",
		want:     nil,
		wantErr:  true,
	}, {
		name:     "does not exist",
		filepath: "../_testdata/values/doesnotexist",
		want:     nil,
		wantErr:  true,
	}, {
		name:     "bad format",
		filepath: "../_testdata/values/bad-format.yaml",
		want:     nil,
		wantErr:  true,
	}, {
		name:     "valid",
		filepath: "../_testdata/values/valid.yaml",
		want: &valuesapi.Values{
			NamespaceSelectors: []valuesapi.NamespaceSelector{{
				Name: "test1",
				Labels: map[string]string{
					"foo.com/managed-state": "managed",
				},
			}},
			Policies: []valuesapi.Policy{{
				Name: "limit-configmap-for-sa",
				Resources: []valuesapi.Resource{{
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
