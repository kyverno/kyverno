package globalcontext

import (
	"context"
	"testing"

	"github.com/kyverno/kyverno/pkg/logging"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"gotest.tools/assert"
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
