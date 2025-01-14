package resource_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/google/cel-go/cel"
	engine "github.com/kyverno/kyverno/pkg/cel"
	"github.com/kyverno/kyverno/pkg/cel/resource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	commoncel "k8s.io/apiserver/pkg/cel"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func TestOpenAPITypeResolver(t *testing.T) {
	c, err := RestConfig()
	if err != nil {
		t.Fatal(err.Error())
	}
	d, err := discovery.NewDiscoveryClientForConfig(c)
	if err != nil {
		t.Fatal(err.Error())
	}

	s := schema.FromAPIVersionAndKind("v1", "ConfigMap")

	resolver := resource.NewOpenAPITypeResolver(d)

	decl, err := resolver.GetDecl(s)
	if err != nil {
		t.Fatal(err.Error())
	}

	env, err := engine.NewEnv()

	provider := commoncel.NewDeclTypeProvider(decl)
	opts, err := provider.EnvOptions(env.CELTypeProvider())

	rootType, _ := provider.FindDeclType("object")
	opts = append(opts, cel.Variable("object", rootType.CelType()))
	env, err = env.Extend(opts...)

	ast, issue := env.Compile(`object.metadata.name != ""`)
	if issue != nil {
		t.Fatal(issue.Err().Error())
	}

	prog, err := env.Program(ast)
	if err != nil {
		t.Fatal(err.Error())
	}

	val, _, err := prog.Eval(map[string]any{
		"object": corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		},
	})
	if err != nil {
		t.Fatal(err.Error())
	}

	fmt.Println(val.Value())
}

func RestConfig() (*rest.Config, error) {
	config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	if err != nil {
		return nil, err
	}
	config.QPS = 300
	config.Burst = 300
	return config, nil
}
