package globalcontext

import (
	"context"
	"strings"
	"testing"

	"github.com/kyverno/kyverno/pkg/logging"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"gotest.tools/v3/assert"
)

func Test_Validate(t *testing.T) {
	type args struct {
		resource []byte
	}
	tc := []struct {
		name    string
		args    args
		want    int
		wantErr bool
	}{
		{
			name: "GlobalContextEntry with both KubernetesResource and APICall present",
			args: args{
				resource: []byte(`{"apiVersion":"kyverno.io/v2alpha1","kind":"GlobalContextEntry","metadata":{"name":"ingress"},"spec":{"apiCall":{"service":{"url":"https://svc.kyverno/example","caBundle":"-----BEGIN CERTIFICATE-----\n-----REDACTED-----\n-----END CERTIFICATE-----"},"refreshInterval":"10ns"},"kubernetesResource":{"group":"apis/networking.k8s.io","version":"v1","resource":"ingresses","namespace":"apps"}}}`),
			},
			want:    0,
			wantErr: true,
		},
		{
			name: "GlobalContextEntry with neither KubernetesResource nor APICall present",
			args: args{
				resource: []byte(`{"apiVersion":"kyverno.io/v2alpha1","kind":"GlobalContextEntry","metadata":{"name":"ingress"},"spec":{}}`),
			},
			want:    0,
			wantErr: true,
		},
		{
			name: "GlobalContextEntry with only KubernetesResource present",
			args: args{
				resource: []byte(`{"apiVersion":"kyverno.io/v2alpha1","kind":"GlobalContextEntry","metadata":{"name":"gce-kubernetesresource"},"spec":{"kubernetesResource":{"group":"apis/networking.k8s.io","version":"v1","resource":"ingresses","namespace":"apps"}}}`),
			},
			want:    0,
			wantErr: false,
		},
		{
			name: "GlobalContextEntry with a core KubernetesResource present",
			args: args{
				resource: []byte(`{"apiVersion":"kyverno.io/v2alpha1","kind":"GlobalContextEntry","metadata":{"name":"gce-kubernetesresource"},"spec":{"kubernetesResource":{"version":"v1","resource":"namespaces"}}}`),
			},
			want:    0,
			wantErr: false,
		},
		{
			name: "GlobalContextEntry with only APICall present",
			args: args{
				resource: []byte(`{"apiVersion":"kyverno.io/v2alpha1","kind":"GlobalContextEntry","metadata":{"name":"gce-apicall"},"spec":{"apiCall":{"service":{"url":"https://svc.kyverno/example","caBundle":"-----BEGIN CERTIFICATE-----\n-----REDACTED-----\n-----END CERTIFICATE-----"},"refreshInterval":"10ns"}}}`),
			},
			want:    0,
			wantErr: false,
		},
		{
			name: "GlobalContextEntry with an expression-only projection passes, apiVersion v2alpha1",
			args: args{
				resource: []byte(`{"apiVersion":"kyverno.io/v2alpha1","kind":"GlobalContextEntry","metadata":{"name":"gce-expression"},"spec":{"apiCall":{"service":{"url":"https://svc.kyverno/example","caBundle":"-----BEGIN CERTIFICATE-----\n-----REDACTED-----\n-----END CERTIFICATE-----"},"refreshInterval":"10ns"},"projections":[{"name":"count","expression":"object.items.size()"}]}}`),
			},
			want:    0,
			wantErr: false,
		},
		{
			name: "GlobalContextEntry with an expression-only projection passes, apiVersion v2beta1",
			args: args{
				resource: []byte(`{"apiVersion":"kyverno.io/v2beta1","kind":"GlobalContextEntry","metadata":{"name":"gce-expression"},"spec":{"apiCall":{"service":{"url":"https://svc.kyverno/example","caBundle":"-----BEGIN CERTIFICATE-----\n-----REDACTED-----\n-----END CERTIFICATE-----"},"refreshInterval":"10ns"},"projections":[{"name":"count","expression":"object.items.size()"}]}}`),
			},
			want:    0,
			wantErr: false,
		},
		{
			name: "GlobalContextEntry with an expression-only projection passes, apiVersion v2",
			args: args{
				resource: []byte(`{"apiVersion":"kyverno.io/v2","kind":"GlobalContextEntry","metadata":{"name":"gce-expression"},"spec":{"apiCall":{"service":{"url":"https://svc.kyverno/example","caBundle":"-----BEGIN CERTIFICATE-----\n-----REDACTED-----\n-----END CERTIFICATE-----"},"refreshInterval":"10ns"},"projections":[{"name":"count","expression":"object.items.size()"}]}}`),
			},
			want:    0,
			wantErr: false,
		},
		{
			name: "GlobalContextEntry with a bad CEL expression fails compilation",
			args: args{
				resource: []byte(`{"apiVersion":"kyverno.io/v2beta1","kind":"GlobalContextEntry","metadata":{"name":"gce-bad-expression"},"spec":{"apiCall":{"service":{"url":"https://svc.kyverno/example","caBundle":"-----BEGIN CERTIFICATE-----\n-----REDACTED-----\n-----END CERTIFICATE-----"},"refreshInterval":"10ns"},"projections":[{"name":"count","expression":"object.items.sizee()"}]}}`),
			},
			want:    0,
			wantErr: true,
		},
		{
			name: "GlobalContextEntry with an http library call fails compilation (library scope excludes http)",
			args: args{
				resource: []byte(`{"apiVersion":"kyverno.io/v2beta1","kind":"GlobalContextEntry","metadata":{"name":"gce-http-scope"},"spec":{"apiCall":{"service":{"url":"https://svc.kyverno/example","caBundle":"-----BEGIN CERTIFICATE-----\n-----REDACTED-----\n-----END CERTIFICATE-----"},"refreshInterval":"10ns"},"projections":[{"name":"count","expression":"http.Get('x')"}]}}`),
			},
			want:    0,
			wantErr: true,
		},
		{
			name: "GlobalContextEntry with a globalContext library call fails compilation (library scope excludes globalContext)",
			args: args{
				resource: []byte(`{"apiVersion":"kyverno.io/v2beta1","kind":"GlobalContextEntry","metadata":{"name":"gce-globalcontext-scope"},"spec":{"apiCall":{"service":{"url":"https://svc.kyverno/example","caBundle":"-----BEGIN CERTIFICATE-----\n-----REDACTED-----\n-----END CERTIFICATE-----"},"refreshInterval":"10ns"},"projections":[{"name":"count","expression":"globalContext.Get('x')"}]}}`),
			},
			want:    0,
			wantErr: true,
		},
	}
	for _, c := range tc {
		t.Run(c.name, func(t *testing.T) {
			gctx, err := admissionutils.UnmarshalGlobalContextEntry(c.args.resource)
			assert.NilError(t, err)
			warnings, err := Validate(context.Background(), logging.GlobalLogger(), gctx)
			if c.wantErr {
				assert.Assert(t, err != nil)
			} else {
				assert.NilError(t, err)
			}
			assert.Assert(t, len(warnings) == c.want)
		})
	}
}

// Test_Validate_BadExpressionReportsFieldPath pins the aggregate error's field path for a
// CEL compile failure on a projection expression: spec.projections[0].expression, matching
// the index/child path the compile loop in validate.go builds.
func Test_Validate_BadExpressionReportsFieldPath(t *testing.T) {
	resource := []byte(`{"apiVersion":"kyverno.io/v2beta1","kind":"GlobalContextEntry","metadata":{"name":"gce-bad-expression"},"spec":{"apiCall":{"service":{"url":"https://svc.kyverno/example","caBundle":"-----BEGIN CERTIFICATE-----\n-----REDACTED-----\n-----END CERTIFICATE-----"},"refreshInterval":"10ns"},"projections":[{"name":"count","expression":"object.items.sizee()"}]}}`)
	gctx, err := admissionutils.UnmarshalGlobalContextEntry(resource)
	assert.NilError(t, err)
	_, err = Validate(context.Background(), logging.GlobalLogger(), gctx)
	assert.Assert(t, err != nil)
	assert.Assert(t, strings.Contains(err.Error(), "spec.projections[0].expression"), "error should name the projection's field path, got: %v", err)
}
