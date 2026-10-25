package externalapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

func TestUpdateStatusUsesStableVersionAndPreservesFields(t *testing.T) {
	t.Parallel()
	ready := false
	gce := &kyvernov2.GlobalContextEntry{
		TypeMeta:   metav1.TypeMeta{APIVersion: "kyverno.io/v2", Kind: "GlobalContextEntry"},
		ObjectMeta: metav1.ObjectMeta{Name: "refresh", Labels: map[string]string{"example.com/keep": "value"}},
		Status:     kyvernov2.GlobalContextEntryStatus{Ready: &ready},
	}
	var mu sync.Mutex
	var requests []string
	var updated kyvernov2.GlobalContextEntry
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		requests = append(requests, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/apis/kyverno.io/v2/globalcontextentries/refresh":
			_ = json.NewEncoder(w).Encode(gce)
		case r.Method == http.MethodPut && r.URL.Path == "/apis/kyverno.io/v2/globalcontextentries/refresh/status":
			if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(&updated)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := kyvernoclient.NewForConfig(&rest.Config{Host: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if err := updateStatus(t.Context(), gce, client); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	want := []string{
		"GET /apis/kyverno.io/v2/globalcontextentries/refresh",
		"PUT /apis/kyverno.io/v2/globalcontextentries/refresh/status",
	}
	if !reflect.DeepEqual(requests, want) {
		t.Fatalf("refresh requests = %v, want %v", requests, want)
	}
	if updated.Status.LastRefreshTime.IsZero() {
		t.Fatal("refresh time was not updated")
	}
	if updated.Status.Ready == nil || *updated.Status.Ready || updated.Labels["example.com/keep"] != "value" {
		t.Fatal("status refresh changed retained readiness or metadata")
	}
	if !gce.Status.LastRefreshTime.IsZero() {
		t.Fatal("status refresh mutated the caller's object")
	}
}
