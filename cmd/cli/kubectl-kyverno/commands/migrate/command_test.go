package migrate

import (
	"bytes"
	"context"
	"testing"

	fatihcolor "github.com/fatih/color"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestCommandRequiresResource(t *testing.T) {
	cmd := Command()
	cmd.SetArgs([]string{})

	err := cmd.Execute()

	require.Error(t, err)
	require.ErrorContains(t, err, `required flag(s) "resource" not set`)
}

func TestCommandRejectsEmptyResource(t *testing.T) {
	cmd := Command()
	cmd.SetArgs([]string{"--resource", "   "})

	err := cmd.Execute()

	require.Error(t, err)
	require.ErrorContains(t, err, "resource cannot be empty")
}

func TestCommand_Flags(t *testing.T) {
	cmd := Command()
	flag := cmd.Flags().Lookup("remove-color")
	require.NotNil(t, flag)
	assert.Equal(t, "false", flag.DefValue)
	assert.Equal(t, "Remove any color from output", flag.Usage)
}

func TestMigrate_ColorAndRemoveColor(t *testing.T) {
	origNoColor := fatihcolor.NoColor
	t.Cleanup(func() {
		fatihcolor.NoColor = origNoColor
		color.Init(origNoColor)
	})

	crdUpToDate := &v1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "myresources.example.com"},
		Spec: v1.CustomResourceDefinitionSpec{
			Group: "example.com",
			Names: v1.CustomResourceDefinitionNames{Plural: "myresources"},
			Scope: v1.ClusterScoped,
			Versions: []v1.CustomResourceDefinitionVersion{
				{Name: "v1", Storage: true},
			},
		},
		Status: v1.CustomResourceDefinitionStatus{
			StoredVersions: []string{"v1"},
		},
	}

	crdNeedsMigration := &v1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "myresources.example.com"},
		Spec: v1.CustomResourceDefinitionSpec{
			Group: "example.com",
			Names: v1.CustomResourceDefinitionNames{Plural: "myresources"},
			Scope: v1.ClusterScoped,
			Versions: []v1.CustomResourceDefinitionVersion{
				{Name: "v1", Storage: false},
				{Name: "v2", Storage: true},
			},
		},
		Status: v1.CustomResourceDefinitionStatus{
			StoredVersions: []string{"v1"},
		},
	}

	item := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "example.com/v2",
			"kind":       "MyResource",
			"metadata": map[string]interface{}{
				"name": "res-1",
			},
		},
	}

	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "example.com", Version: "v2", Resource: "myresources"}: "MyResourceList",
	}

	t.Run("up to date with color enabled", func(t *testing.T) {
		fatihcolor.NoColor = false
		color.Init(false)

		var out bytes.Buffer
		apiClient := apiextensionsfake.NewSimpleClientset(crdUpToDate.DeepCopy())
		dynClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())

		err := migrate(context.Background(), &out, crdUpToDate.DeepCopy(), dynClient, apiClient)
		require.NoError(t, err)

		output := out.String()
		assert.Contains(t, output, "\x1b[36;1mv1\x1b[0;22m")
		assert.Contains(t, output, "\x1b[32;1mstored version is already up to date, nothing to do\x1b[0;22m")
	})

	t.Run("up to date with remove-color", func(t *testing.T) {
		fatihcolor.NoColor = false
		color.Init(true)

		var out bytes.Buffer
		apiClient := apiextensionsfake.NewSimpleClientset(crdUpToDate.DeepCopy())
		dynClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())

		err := migrate(context.Background(), &out, crdUpToDate.DeepCopy(), dynClient, apiClient)
		require.NoError(t, err)

		output := out.String()
		assert.Contains(t, output, "stored version: v1")
		assert.Contains(t, output, "stored version is already up to date, nothing to do")
		assert.NotContains(t, output, "\x1b[")
	})

	t.Run("migration with color enabled", func(t *testing.T) {
		fatihcolor.NoColor = false
		color.Init(false)

		var out bytes.Buffer
		crd := crdNeedsMigration.DeepCopy()
		apiClient := apiextensionsfake.NewSimpleClientset(crd)
		dynClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrToListKind, item.DeepCopy())

		err := migrate(context.Background(), &out, crd, dynClient, apiClient)
		require.NoError(t, err)

		output := out.String()
		assert.Contains(t, output, "\x1b[36;1mv2\x1b[0;22m")
		assert.Contains(t, output, "\x1b[33;1mmigrating resources...\x1b[0;22m")
		assert.Contains(t, output, "\x1b[33;1mpatching status...\x1b[0;22m")
	})

	t.Run("migration with remove-color", func(t *testing.T) {
		fatihcolor.NoColor = false
		color.Init(true)

		var out bytes.Buffer
		crd := crdNeedsMigration.DeepCopy()
		apiClient := apiextensionsfake.NewSimpleClientset(crd)
		dynClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrToListKind, item.DeepCopy())

		err := migrate(context.Background(), &out, crd, dynClient, apiClient)
		require.NoError(t, err)

		output := out.String()
		assert.Contains(t, output, "stored version: v2")
		assert.Contains(t, output, "migrating resources...")
		assert.Contains(t, output, "patching status...")
		assert.NotContains(t, output, "\x1b[")
	})
}
