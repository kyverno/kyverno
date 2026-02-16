package yaml

import (
	"reflect"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/libs/versions"
	"k8s.io/apimachinery/pkg/util/version"
)

type mockYamlIface struct{}

func (m *mockYamlIface) Parse(s []byte) (any, error) {
	return map[string]interface{}{"key": "value"}, nil
}

func TestLatest(t *testing.T) {
	if got := Latest(); !reflect.DeepEqual(got, versions.YamlVersion) {
		t.Errorf("Latest() = %v, want %v", got, versions.YamlVersion)
	}
}

func TestLib(t *testing.T) {
	v := version.MustParseSemantic("1.0.0")
	l := Lib(&mockYamlIface{}, v)
	if l == nil {
		t.Error("Lib() returned nil")
	}
}

func TestLibrary_Structure(t *testing.T) {
	v := version.MustParseSemantic("1.0.0")

	l := &lib{
		yaml:    Yaml{&mockYamlIface{}},
		version: v,
	}

	if name := l.LibraryName(); name != "kyverno.yaml" {
		t.Errorf("LibraryName() = %q, want %q", name, "kyverno.yaml")
	}

	progOpts := l.ProgramOptions()
	if len(progOpts) == 0 {
		t.Error("ProgramOptions() returned empty slice")
	}

	compOpts := l.CompileOptions()
	if len(compOpts) == 0 {
		t.Error("CompileOptions() returned empty slice")
	}
}

func TestLibrary_Integration(t *testing.T) {
	mock := &mockYamlIface{}
	v := version.MustParseSemantic("1.0.0")

	env, err := cel.NewEnv(Lib(mock, v))
	if err != nil {
		t.Fatalf("Failed to create CEL env: %v", err)
	}

	ast, issues := env.Compile(`yaml.parse('data: test')`)
	if issues != nil && issues.Err() != nil {
		t.Fatalf("CEL compilation failed: %v", issues.Err())
	}

	prog, err := env.Program(ast)
	if err != nil {
		t.Fatalf("Failed to create program: %v", err)
	}

	out, _, err := prog.Eval(map[string]interface{}{})
	if err != nil {
		t.Fatalf("CEL evaluation failed: %v", err)
	}

	val, ok := out.(ref.Val)
	if !ok {
		t.Fatalf("Output is not a ref.Val")
	}

	native, err := val.ConvertToNative(reflect.TypeOf(map[string]interface{}{}))
	if err != nil {
		t.Fatalf("Failed to convert output to native map: %v", err)
	}

	resultMap := native.(map[string]interface{})
	if resultMap["key"] != "value" {
		t.Errorf("Expected result['key']='value', got %v", resultMap["key"])
	}
}
