package loader

import (
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/data"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/openapi"
	"sigs.k8s.io/kubectl-validate/pkg/openapiclient"
	"sigs.k8s.io/kubectl-validate/pkg/validator"
	"sigs.k8s.io/yaml"
)

type errClient struct{}

func (errClient) Paths() (map[string]openapi.GroupVersion, error) {
	return nil, errors.New("error")
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		client  openapi.Client
		want    Loader
		wantErr bool
	}{{
		name:    "err client",
		client:  errClient{},
		wantErr: true,
	}, {
		name:   "builtin",
		client: openapiclient.NewHardcodedBuiltins("1.27"),
		want: func() Loader {
			validator, err := validator.New(openapiclient.NewHardcodedBuiltins("1.27"))
			require.NoError(t, err)
			return &loader{
				validator: validator,
			}
		}(),
	}, {
		name:    "invalid local",
		client:  openapiclient.NewLocalCRDFiles(data.Crds(), "blam"),
		wantErr: true,
	}, {
		name:   "composite - no clients",
		client: openapiclient.NewComposite(),
		want: func() Loader {
			validator, err := validator.New(openapiclient.NewComposite())
			require.NoError(t, err)
			return &loader{
				validator: validator,
			}
		}(),
	}, {
		name:    "composite - err client",
		client:  openapiclient.NewComposite(errClient{}),
		wantErr: true,
	}, {
		name:    "composite - with err client",
		client:  openapiclient.NewComposite(openapiclient.NewHardcodedBuiltins("1.27"), errClient{}),
		wantErr: true,
	}, {
		name:    "composite - invalid local",
		client:  openapiclient.NewComposite(openapiclient.NewLocalCRDFiles(data.Crds(), "blam")),
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.client)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("New() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_loader_Load(t *testing.T) {
	loadFile := func(path string) []byte {
		bytes, err := os.ReadFile(path)
		require.NoError(t, err)
		return bytes
	}
	newLoader := func(client openapi.Client) Loader {
		loader, err := New(client)
		require.NoError(t, err)
		return loader
	}
	toUnstructured := func(data []byte) unstructured.Unstructured {
		json, err := yaml.YAMLToJSON(data)
		require.NoError(t, err)
		var result unstructured.Unstructured
		require.NoError(t, result.UnmarshalJSON(json))
		if result.GetCreationTimestamp().Time.IsZero() {
			require.NoError(t, unstructured.SetNestedField(result.UnstructuredContent(), nil, "metadata", "creationTimestamp"))
		}
		return result
	}
	tests := []struct {
		name     string
		loader   Loader
		document []byte
		want     unstructured.Unstructured
		wantErr  bool
	}{{
		name:    "nil",
		loader:  newLoader(openapiclient.NewLocalCRDFiles(data.Crds(), "crds")),
		wantErr: true,
	}, {
		name:     "empty GVK",
		loader:   newLoader(openapiclient.NewLocalCRDFiles(data.Crds(), "crds")),
		document: []byte(`foo: bar`),
		wantErr:  true,
	}, {
		name:   "not yaml",
		loader: newLoader(openapiclient.NewLocalCRDFiles(data.Crds(), "crds")),
		document: []byte(`
		foo
		  bar
		  - baz`),
		wantErr: true,
	}, {
		name:     "unknown GVK",
		loader:   newLoader(openapiclient.NewLocalCRDFiles(data.Crds(), "crds")),
		document: loadFile("../../_testdata/resources/namespace.yaml"),
		wantErr:  true,
	}, {
		name:   "bad schema",
		loader: newLoader(openapiclient.NewHardcodedBuiltins("1.27")),
		document: []byte(`
		apiVersion: v1
		kind: Namespace
		bad: field
		metadata:
		  name: prod-bus-app1
		  labels:
		    purpose: production`),
		wantErr: true,
	}, {
		name:     "ok",
		loader:   newLoader(openapiclient.NewHardcodedBuiltins("1.27")),
		document: loadFile("../../_testdata/resources/namespace.yaml"),
		want:     toUnstructured(loadFile("../../_testdata/resources/namespace.yaml")),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got, err := tt.loader.Load(tt.document)
			if (err != nil) != tt.wantErr {
				t.Errorf("loader.Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("loader.Load() = %v, want %v", got, tt.want)
			}
		})
	}
}
