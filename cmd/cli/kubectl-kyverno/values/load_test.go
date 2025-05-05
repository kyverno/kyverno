package values

import (
	"os"
	"reflect"
	"testing"
	
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
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
		filepath: "../_testdata/values/limit-configmap-for-sa.yaml",
		want:     mustReadFile("../_testdata/values/limit-configmap-for-sa.yaml"),
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
			defer file.Close()
			if _, err := file.Write([]byte("foo: bar")); err != nil {
				t.Fatal(err)
			}
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
		want     *v1alpha1.Values
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
		filepath: "../_testdata/values/limit-configmap-for-sa.yaml",
		want: &v1alpha1.Values{
			ValuesSpec: v1alpha1.ValuesSpec{
				NamespaceSelectors: []v1alpha1.NamespaceSelector{{
					Name: "test1",
					Labels: map[string]string{
						"foo.com/managed-state": "managed",
					},
				}},
				Policies: []v1alpha1.Policy{{
					Name: "limit-configmap-for-sa",
					Resources: []v1alpha1.Resource{{
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
func TestLoad_ConfigMap(t *testing.T) {
	fs := memfs.New()

	cm := corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{Kind: "ConfigMap", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
		},
		Data: map[string]string{
			"FOO": "bar",
			"NUM": "42",
		},
	}
	yamlBytes, err := yaml.Marshal(cm)
	require.NoError(t, err)
	file, err := fs.Create("cm.yaml")
	require.NoError(t, err)
	defer file.Close()
	_, err = file.Write(yamlBytes)
	require.NoError(t, err)
	
	vals, err := Load(fs, "/cm.yaml")
	require.NoError(t, err)

	assert.Equal(t, "Values", vals.TypeMeta.Kind)
	assert.Equal(t, "default", vals.ObjectMeta.Namespace)
	assert.Equal(t, "bar", vals.ValuesSpec.GlobalValues["FOO"])
	assert.Equal(t, "42", vals.ValuesSpec.GlobalValues["NUM"])
}