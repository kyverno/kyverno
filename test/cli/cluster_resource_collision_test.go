package cli_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestApplyClusterResourceCollisions runs the built CLI against discovery and resource fixtures, including its process exit status.
func TestApplyClusterResourceCollisions(t *testing.T) {
	binary := os.Getenv("KYVERNO_CLI_BIN")
	if binary == "" {
		t.Skip("run make test-cli-local-cluster to build and exercise the CLI")
	}
	fixtureDir := "cluster-resource-collision"
	fixture, err := os.ReadFile(filepath.Join(fixtureDir, "resources.json"))
	if err != nil {
		t.Fatal(err)
	}
	var resources struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(fixture, &resources); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		write := func(value any) {
			if err := json.NewEncoder(w).Encode(value); err != nil {
				t.Errorf("mock API response: %v", err)
			}
		}
		switch r.URL.Path {
		case "/api":
			write(map[string]any{"kind": "APIVersions", "apiVersion": "v1", "versions": []string{"v1"}})
			return
		case "/apis":
			groups := []metav1.APIGroup{}
			for _, group := range []string{"alpha.example.com", "beta.example.com"} {
				version := metav1.GroupVersionForDiscovery{GroupVersion: group + "/v1", Version: "v1"}
				groups = append(groups, metav1.APIGroup{Name: group, Versions: []metav1.GroupVersionForDiscovery{version}, PreferredVersion: version})
			}
			write(metav1.APIGroupList{TypeMeta: metav1.TypeMeta{Kind: "APIGroupList", APIVersion: "v1"}, Groups: groups})
			return
		case "/api/v1":
			write(metav1.APIResourceList{
				GroupVersion: "v1", APIResources: []metav1.APIResource{{Name: "namespaces", Kind: "Namespace", Verbs: []string{"get", "list", "watch"}}},
			})
			return
		case "/api/v1/namespaces":
			if r.URL.Query().Get("watch") == "true" {
				w.(http.Flusher).Flush()
				<-r.Context().Done()
				return
			}
			write(map[string]any{"apiVersion": "v1", "kind": "NamespaceList", "metadata": map[string]any{"resourceVersion": "1"}, "items": []any{
				map[string]any{"apiVersion": "v1", "kind": "Namespace", "metadata": map[string]any{"name": "default", "resourceVersion": "1"}},
			}})
			return
		}
		for i, group := range []string{"alpha.example.com", "beta.example.com"} {
			base := "/apis/" + group + "/v1"
			if r.URL.Path == base {
				write(metav1.APIResourceList{GroupVersion: group + "/v1", APIResources: []metav1.APIResource{
					{Name: "widgets", Kind: "Widget", Namespaced: true, Verbs: []string{"get", "list"}},
					{Name: "widgets/status", Kind: "Widget", Namespaced: true, Verbs: []string{"get"}},
					{Name: "widgets/resize", Kind: "Widget", Namespaced: true, Verbs: []string{"get"}},
				}})
				return
			}
			listPath := base + "/namespaces/default/widgets"
			if r.URL.Path == listPath || r.URL.Path == base+"/widgets" {
				write(map[string]any{"apiVersion": group + "/v1", "kind": "WidgetList", "items": []json.RawMessage{resources.Items[i]}})
				return
			}
			if r.URL.Path == listPath+"/shared/status" || r.URL.Path == listPath+"/shared/resize" {
				var resource map[string]any
				if err := json.Unmarshal(resources.Items[i], &resource); err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				resource["spec"] = map[string]any{"allowed": strings.HasSuffix(r.URL.Path, "/resize")}
				write(resource)
				return
			}
		}
		t.Errorf("unexpected mock API request: %s %s", r.Method, r.URL)
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)
	kubeconfig := filepath.Join(t.TempDir(), "config")
	config := fmt.Sprintf("apiVersion: v1\nkind: Config\nclusters:\n- name: mock\n  cluster:\n    server: %s\ncontexts:\n- name: mock\n  context:\n    cluster: mock\n    user: mock\ncurrent-context: mock\nusers:\n- name: mock\n  user: {}\n", server.URL)
	if err := os.WriteFile(kubeconfig, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		name   string
		policy string
		count  int
		passes int
	}{
		{name: "cross-group", policy: "policy.yaml", count: 2, passes: 1},
		{name: "parent-and-subresources", policy: "subresources-policy.yaml", count: 3, passes: 2},
	} {
		for _, concurrency := range []int{1, 2, 4} {
			t.Run(fmt.Sprintf("%s/concurrency=%d", scenario.name, concurrency), func(t *testing.T) {
				ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
				defer cancel()
				// The executable is supplied by the Makefile target, never by API responses or fixtures.
				cmd := exec.CommandContext(ctx, binary, "apply", filepath.Join(fixtureDir, scenario.policy), "--cluster", "--kubeconfig", kubeconfig, "--namespace", "default", "--concurrent", fmt.Sprint(concurrency), "--remove-color") // #nosec G204
				output, err := cmd.CombinedOutput()
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
					t.Fatalf("expected policy violation exit status 1, got %v\n%s", err, output)
				}
				for _, expected := range []string{
					fmt.Sprintf("Applying 1 policy rule(s) to %d resource(s)", scenario.count),
					fmt.Sprintf("pass: %d, fail: 1, warn: 0, error: 0, skip: 0", scenario.passes),
					"widget must be allowed",
				} {
					if !strings.Contains(string(output), expected) {
						t.Errorf("missing %q in CLI output:\n%s", expected, output)
					}
				}
			})
		}
	}
}
