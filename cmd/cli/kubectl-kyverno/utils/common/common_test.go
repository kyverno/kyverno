package common

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cli/loader"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Test_GetGitBranchOrPolicyPaths(t *testing.T) {
	type TestCase struct {
		gitBranch                             string
		repoURL                               string
		policyPath                            string
		desiredBranch, actualBranch           string
		desiredPathToYAMLs, actualPathToYAMLs string
	}
	testcases := []TestCase{
		{
			gitBranch:          "main",
			repoURL:            "https://github.com/kyverno/policies",
			policyPath:         "https://github.com/kyverno/policies/openshift/team-validate-ns-name/",
			desiredBranch:      "main",
			desiredPathToYAMLs: "/openshift/team-validate-ns-name/",
		},
		{
			gitBranch:          "",
			repoURL:            "https://github.com/kyverno/policies",
			policyPath:         "https://github.com/kyverno/policies/",
			desiredBranch:      "main",
			desiredPathToYAMLs: "/",
		},
		{
			gitBranch:          "",
			repoURL:            "https://github.com/kyverno/policies",
			policyPath:         "https://github.com/kyverno/policies",
			desiredBranch:      "main",
			desiredPathToYAMLs: "/",
		},
	}

	for _, tc := range testcases {
		tc.actualBranch, tc.actualPathToYAMLs = GetGitBranchOrPolicyPaths(tc.gitBranch, tc.repoURL, tc.policyPath)
		if tc.actualBranch != tc.desiredBranch || tc.actualPathToYAMLs != tc.desiredPathToYAMLs {
			t.Errorf("Want %q got %q  OR Want %q got %q", tc.desiredBranch, tc.actualBranch, tc.desiredPathToYAMLs, tc.actualPathToYAMLs)
		}
	}
}

func TestGetResourceAccordingToResourcePath_MultipleDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	dirA := filepath.Join(tmpDir, "team-a")
	dirB := filepath.Join(tmpDir, "team-b")
	require.NoError(t, os.Mkdir(dirA, 0o755))
	require.NoError(t, os.Mkdir(dirB, 0o755))

	cmA := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm-a\n  namespace: default\n"
	cmB := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm-b\n  namespace: default\n"
	require.NoError(t, os.WriteFile(filepath.Join(dirA, "good.yaml"), []byte(cmA), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dirB, "bad.yaml"), []byte(cmB), 0o644))

	resources, err := GetResourceAccordingToResourcePath(
		io.Discard, nil, []string{dirA, dirB},
		false, nil, nil, "", false, false, "",
		loader.ResourceOptions{}, false,
	)
	require.NoError(t, err)
	require.Len(t, resources, 2)
	assert.Equal(t, "cm-a", resources[0].GetName())
	assert.Equal(t, "cm-b", resources[1].GetName())
}

func TestGetResourceAccordingToResourcePath_MixedFilesAndDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "cm-1.yaml")
	dir := filepath.Join(tmpDir, "nested-dir")
	file2 := filepath.Join(tmpDir, "cm-2.yaml")
	require.NoError(t, os.Mkdir(dir, 0o755))

	cm1 := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm-1\n  namespace: default\n"
	cmDir := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm-dir\n  namespace: default\n"
	cm2 := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm-2\n  namespace: default\n"
	require.NoError(t, os.WriteFile(file1, []byte(cm1), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cm-dir.yaml"), []byte(cmDir), 0o644))
	require.NoError(t, os.WriteFile(file2, []byte(cm2), 0o644))

	resources, err := GetResourceAccordingToResourcePath(
		io.Discard, nil, []string{file1, dir, file2},
		false, nil, nil, "", false, false, "",
		loader.ResourceOptions{}, false,
	)
	require.NoError(t, err)
	require.Len(t, resources, 3)
	assert.Equal(t, "cm-1", resources[0].GetName())
	assert.Equal(t, "cm-dir", resources[1].GetName())
	assert.Equal(t, "cm-2", resources[2].GetName())
}

func TestGetResourceAccordingToResourcePath_DirectoryFiltersNonYAMLAndSubdirs(t *testing.T) {
	tmpDir := t.TempDir()

	dir := filepath.Join(tmpDir, "resources")
	require.NoError(t, os.Mkdir(dir, 0o755))

	// Nested subdirectory with a .yaml extension should NOT be treated as a YAML file
	subDir := filepath.Join(dir, "sub.yaml")
	require.NoError(t, os.Mkdir(subDir, 0o755))

	cmYaml := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm-yaml\n  namespace: default\n"
	cmYml := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm-yml\n  namespace: default\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cm.yaml"), []byte(cmYaml), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cm.yml"), []byte(cmYml), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("notes"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}"), 0o644))

	resources, err := GetResourceAccordingToResourcePath(
		io.Discard, nil, []string{dir},
		false, nil, nil, "", false, false, "",
		loader.ResourceOptions{}, false,
	)
	require.NoError(t, err)
	require.Len(t, resources, 2)
	names := []string{resources[0].GetName(), resources[1].GetName()}
	assert.Contains(t, names, "cm-yaml")
	assert.Contains(t, names, "cm-yml")
}

func TestGetResourceAccordingToResourcePath_MixedURLAndDirectory(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write([]byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm-url\n  namespace: default\n"))
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "local-dir")
	require.NoError(t, os.Mkdir(dir, 0o755))
	cmDir := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm-dir\n  namespace: default\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cm-dir.yaml"), []byte(cmDir), 0o644))

	t.Run("URL followed by directory", func(t *testing.T) {
		resources, err := GetResourceAccordingToResourcePath(
			io.Discard, nil, []string{ts.URL, dir},
			false, nil, nil, "", false, false, "",
			loader.ResourceOptions{}, false,
		)
		require.NoError(t, err)
		require.Len(t, resources, 2)
		assert.Equal(t, "cm-url", resources[0].GetName())
		assert.Equal(t, "cm-dir", resources[1].GetName())
	})

	t.Run("directory followed by URL", func(t *testing.T) {
		resources, err := GetResourceAccordingToResourcePath(
			io.Discard, nil, []string{dir, ts.URL},
			false, nil, nil, "", false, false, "",
			loader.ResourceOptions{}, false,
		)
		require.NoError(t, err)
		require.Len(t, resources, 2)
		assert.Equal(t, "cm-dir", resources[0].GetName())
		assert.Equal(t, "cm-url", resources[1].GetName())
	})
}

func TestGetResourceAccordingToResourcePath_InvalidOrUnreachableURL(t *testing.T) {
	notFoundServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer notFoundServer.Close()

	tmpDir := t.TempDir()
	validDir := filepath.Join(tmpDir, "valid-dir")
	require.NoError(t, os.Mkdir(validDir, 0o755))
	cm := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm-valid\n  namespace: default\n"
	require.NoError(t, os.WriteFile(filepath.Join(validDir, "cm.yaml"), []byte(cm), 0o644))

	testCases := []struct {
		name  string
		paths []string
	}{
		{
			name:  "http 404 url alone",
			paths: []string{notFoundServer.URL + "/missing.yaml"},
		},
		{
			name:  "valid directory followed by http 404 url",
			paths: []string{validDir, notFoundServer.URL + "/missing.yaml"},
		},
		{
			name:  "malformed http url",
			paths: []string{"http://[::1]:namedport/resource.yaml"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resources, err := GetResourceAccordingToResourcePath(
				io.Discard, nil, tc.paths,
				false, nil, nil, "", false, false, "",
				loader.ResourceOptions{}, false,
			)
			require.Error(t, err)
			assert.Nil(t, resources)
		})
	}
}

func TestGetResourceAccordingToResourcePath_NonExistentPath(t *testing.T) {
	tmpDir := t.TempDir()
	validDir := filepath.Join(tmpDir, "valid-dir")
	require.NoError(t, os.Mkdir(validDir, 0o755))
	cm := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm-valid\n  namespace: default\n"
	require.NoError(t, os.WriteFile(filepath.Join(validDir, "cm.yaml"), []byte(cm), 0o644))

	nonExistent := filepath.Join(tmpDir, "does-not-exist")

	testCases := []struct {
		name  string
		paths []string
	}{
		{
			name:  "missing path only",
			paths: []string{nonExistent},
		},
		{
			name:  "valid directory followed by missing path",
			paths: []string{validDir, nonExistent},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := GetResourceAccordingToResourcePath(
				io.Discard, nil, tc.paths,
				false, nil, nil, "", false, false, "",
				loader.ResourceOptions{}, false,
			)
			require.Error(t, err)
			assert.True(t, os.IsNotExist(err))
		})
	}
}

func TestGetResourceAccordingToResourcePath_ClusterModePreservesSelectors(t *testing.T) {
	// In cluster mode with a cluster client (cluster == true && dClient != nil),
	// ResourcePaths are cluster resource selectors, not local files/directories.
	pod1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "my-test-pod",
				"namespace": "default",
			},
		},
	}
	pod2 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "other-pod",
				"namespace": "default",
			},
		},
	}
	dClient, err := dclient.NewFakeClient(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{
			{Group: "", Version: "v1", Resource: "pods"}: "PodList",
		},
		pod1,
		pod2,
	)
	require.NoError(t, err)
	dClient.SetDiscovery(dclient.NewFakeDiscoveryClient([]schema.GroupVersionResource{
		{Group: "", Version: "v1", Resource: "pods"},
	}))

	policies := []engineapi.GenericPolicy{
		engineapi.NewMutatingPolicy(&v1beta1.MutatingPolicy{
			Spec: v1beta1.MutatingPolicySpec{
				MatchConstraints: makeMatchResources("", "pods"),
			},
		}),
	}

	t.Run("multiple cluster selectors fetch matching resources", func(t *testing.T) {
		resources, err := GetResourceAccordingToResourcePath(
			io.Discard, nil, []string{"my-test-pod", "manifests/other-pod.yaml"},
			true, policies, dClient, "", false, false, "",
			loader.ResourceOptions{}, false,
		)
		require.NoError(t, err)
		require.Len(t, resources, 2)
		assert.Equal(t, "my-test-pod", resources[0].GetName())
		assert.Equal(t, "other-pod", resources[1].GetName())
	})

	t.Run("missing cluster selector returns cluster not-found error instead of os.IsNotExist", func(t *testing.T) {
		resources, err := GetResourceAccordingToResourcePath(
			io.Discard, nil, []string{"my-test-pod", "manifests/missing-pod.yaml"},
			true, policies, dClient, "", false, false, "",
			loader.ResourceOptions{}, false,
		)
		require.Error(t, err)
		assert.Nil(t, resources)
		assert.False(t, os.IsNotExist(err), "cluster mode with client should not attempt local file os.Stat")
		assert.Contains(t, err.Error(), "manifests/missing-pod.yaml not found in cluster")
	})
}

func TestGetResourceAccordingToResourcePath_ClusterModeWithNilClientExpandsDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "resources")
	require.NoError(t, os.Mkdir(dir, 0o755))

	cm1 := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: target-cm1\n  namespace: default\n"
	cm2 := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: target-cm2\n  namespace: default\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cm1.yaml"), []byte(cm1), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cm2.yml"), []byte(cm2), 0o644))

	// In cluster mode, target and parameter resources are loaded before dClient is initialized (dClient == nil).
	// Directory expansion must still expand local directories in this case.
	resources, err := GetResourceAccordingToResourcePath(
		io.Discard, nil, []string{dir},
		true, nil, nil, "", false, false, "",
		loader.ResourceOptions{}, false,
	)
	require.NoError(t, err)
	require.Len(t, resources, 2)
	names := []string{resources[0].GetName(), resources[1].GetName()}
	assert.Contains(t, names, "target-cm1")
	assert.Contains(t, names, "target-cm2")
}

func TestGetResourceAccordingToResourcePath_EmptyPaths(t *testing.T) {
	resources, err := GetResourceAccordingToResourcePath(
		io.Discard, nil, []string{},
		false, nil, nil, "", false, false, "",
		loader.ResourceOptions{}, false,
	)
	require.NoError(t, err)
	assert.Empty(t, resources)
}
