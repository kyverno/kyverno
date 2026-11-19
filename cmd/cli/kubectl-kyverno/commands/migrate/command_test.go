package migrate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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
	ktesting "k8s.io/client-go/testing"
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
	t.Setenv("NO_COLOR", "")
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

func createTestKubeconfig(t *testing.T, serverURL string) string {
	config := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- cluster:
    server: %s
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
`, serverURL)
	tmpFile, err := os.CreateTemp(t.TempDir(), "kubeconfig-*.yaml")
	require.NoError(t, err)
	_, err = tmpFile.WriteString(config)
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())
	return tmpFile.Name()
}

func TestCommand_ColorAndRemoveColor_EndToEnd(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	origNoColor := fatihcolor.NoColor
	t.Cleanup(func() {
		fatihcolor.NoColor = origNoColor
		color.Init(origNoColor)
	})

	crd := &v1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		},
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

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(crd)
	}))
	t.Cleanup(server.Close)

	kubeconfigFile := createTestKubeconfig(t, server.URL)

	t.Run("default color enabled through Command RunE", func(t *testing.T) {
		fatihcolor.NoColor = false
		color.Init(false)

		cmd := Command()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"--kubeconfig", kubeconfigFile, "--resource", "myresources.example.com"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := out.String()
		assert.Contains(t, output, "\x1b[36;1mmyresources.example.com\x1b[0;22m")
		assert.Contains(t, output, "\x1b[36;1mv1\x1b[0;22m")
		assert.Contains(t, output, "\x1b[32;1mstored version is already up to date, nothing to do\x1b[0;22m")
	})

	t.Run("remove-color flag through Command RunE", func(t *testing.T) {
		fatihcolor.NoColor = false
		color.Init(false)

		cmd := Command()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"--kubeconfig", kubeconfigFile, "--resource", "myresources.example.com", "--remove-color"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := out.String()
		assert.Contains(t, output, "migrating resource: myresources.example.com ...")
		assert.Contains(t, output, "stored version: v1")
		assert.Contains(t, output, "stored version is already up to date, nothing to do")
		assert.NotContains(t, output, "\x1b[")
	})
}

func TestMigrate_FailurePaths(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	origNoColor := fatihcolor.NoColor
	t.Cleanup(func() {
		fatihcolor.NoColor = origNoColor
		color.Init(origNoColor)
	})

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

	t.Run("list failure with color enabled", func(t *testing.T) {
		fatihcolor.NoColor = false
		color.Init(false)

		var out bytes.Buffer
		crd := crdNeedsMigration.DeepCopy()
		apiClient := apiextensionsfake.NewSimpleClientset(crd)
		dynClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrToListKind, item.DeepCopy())
		dynClient.PrependReactor("list", "myresources", func(action ktesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("injected list failure")
		})

		err := migrate(context.Background(), &out, crd, dynClient, apiClient)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "injected list failure")

		output := out.String()
		assert.Contains(t, output, "\x1b[36;1mv2\x1b[0;22m")
		assert.Contains(t, output, "\x1b[33;1mmigrating resources...\x1b[0;22m")
		assert.NotContains(t, output, "patching status...")

		for _, action := range apiClient.Actions() {
			assert.False(t, action.Matches("update", "customresourcedefinitions"))
		}
	})

	t.Run("list failure with remove-color", func(t *testing.T) {
		fatihcolor.NoColor = false
		color.Init(true)

		var out bytes.Buffer
		crd := crdNeedsMigration.DeepCopy()
		apiClient := apiextensionsfake.NewSimpleClientset(crd)
		dynClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrToListKind, item.DeepCopy())
		dynClient.PrependReactor("list", "myresources", func(action ktesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("injected list failure")
		})

		err := migrate(context.Background(), &out, crd, dynClient, apiClient)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "injected list failure")

		output := out.String()
		assert.Contains(t, output, "stored version: v2")
		assert.Contains(t, output, "migrating resources...")
		assert.NotContains(t, output, "patching status...")
		assert.NotContains(t, output, "\x1b[")

		for _, action := range apiClient.Actions() {
			assert.False(t, action.Matches("update", "customresourcedefinitions"))
		}
	})

	t.Run("update failure with color enabled", func(t *testing.T) {
		fatihcolor.NoColor = false
		color.Init(false)

		var out bytes.Buffer
		crd := crdNeedsMigration.DeepCopy()
		apiClient := apiextensionsfake.NewSimpleClientset(crd)
		dynClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrToListKind, item.DeepCopy())
		dynClient.PrependReactor("update", "myresources", func(action ktesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("injected update failure")
		})

		err := migrate(context.Background(), &out, crd, dynClient, apiClient)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "injected update failure")

		output := out.String()
		assert.Contains(t, output, "\x1b[36;1mv2\x1b[0;22m")
		assert.Contains(t, output, "\x1b[33;1mmigrating resources...\x1b[0;22m")
		assert.NotContains(t, output, "patching status...")

		for _, action := range apiClient.Actions() {
			assert.False(t, action.Matches("update", "customresourcedefinitions"))
		}
	})

	t.Run("update failure with remove-color", func(t *testing.T) {
		fatihcolor.NoColor = false
		color.Init(true)

		var out bytes.Buffer
		crd := crdNeedsMigration.DeepCopy()
		apiClient := apiextensionsfake.NewSimpleClientset(crd)
		dynClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrToListKind, item.DeepCopy())
		dynClient.PrependReactor("update", "myresources", func(action ktesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("injected update failure")
		})

		err := migrate(context.Background(), &out, crd, dynClient, apiClient)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "injected update failure")

		output := out.String()
		assert.Contains(t, output, "stored version: v2")
		assert.Contains(t, output, "migrating resources...")
		assert.NotContains(t, output, "patching status...")
		assert.NotContains(t, output, "\x1b[")

		for _, action := range apiClient.Actions() {
			assert.False(t, action.Matches("update", "customresourcedefinitions"))
		}
	})
}
