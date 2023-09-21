package resource

import (
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/resource/loader"
	"sigs.k8s.io/kubectl-validate/pkg/openapiclient"
)

const (
	singleResource string = `apiVersion: v1
kind: Namespace
metadata:
  name: prod-bus-app1
  labels:
    purpose: production`

	multipleResources string = `
apiVersion: v1
kind: Pod
metadata:
  labels:
    run: nginx
  name: nginx
  namespace: default
spec:
  containers:
    - image: nginx
      name: nginx
      resources: {}
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    run: redis
  name: redis
  namespace: default
spec:
  containers:
    - image: redis
      name: redis
      resources: {}`

	resourceWithComment string = `
### POD ###
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    run: nginx
  name: nginx
  namespace: default
spec:
  containers:
    - image: nginx
      name: nginx
      resources: {}`
)

func Test_LoadResources(t *testing.T) {
	l, err := loader.New(openapiclient.NewHardcodedBuiltins("1.27"))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name       string
		resources  string
		wantLoaded int
		wantErr    bool
	}{
		{
			name:       "load no resource with empy string",
			resources:  "",
			wantLoaded: 0,
			wantErr:    false,
		},
		{
			name:       "load single resource",
			resources:  singleResource,
			wantLoaded: 1,
			wantErr:    false,
		},
		{
			name:       "load multiple resources",
			resources:  multipleResources,
			wantLoaded: 2,
			wantErr:    false,
		},
		{
			name:       "load resource with comment",
			resources:  resourceWithComment,
			wantLoaded: 1,
			wantErr:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if res, err := LoadResources(l, []byte(tt.resources)); (err != nil) != tt.wantErr {
				t.Errorf("loader.Resources() error = %v, wantErr %v", err, tt.wantErr)
			} else if len(res) != tt.wantLoaded {
				t.Errorf("loader.Resources() loaded amount = %v, wantLoaded %v", len(res), tt.wantLoaded)
			}
		})
	}
}
