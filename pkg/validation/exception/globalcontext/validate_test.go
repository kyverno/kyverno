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
		opts     ValidationOptions
		resource []byte
	}
	tc := []struct {
		name    string
		args    args
		want    int
		wantErr bool
	}{
		{
			name: "GlobalContextEntry disabled.",
			args: args{
				opts: ValidationOptions{
					Enabled: false,
				},
				resource: []byte(`{"apiVersion":"kyverno.io/v2alpha1","kind":"GlobalContextEntry","metadata":{"name":"ingress"},"spec":{"apiCall":{"service":{"url":"https://svc.kyverno/example","caBundle":"-----BEGIN CERTIFICATE-----\n-----REDACTED-----\n-----END CERTIFICATE-----"},"refreshInterval":"10ns"}}}`),
			},
			want:    1,
			wantErr: false,
		},
		{
			name: "GlobalContextEntry enabled, both KubernetesResource and APICall present",
			args: args{
				opts: ValidationOptions{
					Enabled: true,
				},
				resource: []byte(`{"apiVersion":"kyverno.io/v2alpha1","kind":"GlobalContextEntry","metadata":{"name":"ingress"},"spec":{"apiCall":{"service":{"url":"https://svc.kyverno/example","caBundle":"-----BEGIN CERTIFICATE-----\n-----REDACTED-----\n-----END CERTIFICATE-----"},"refreshInterval":"10ns"},"kubernetesResource":{"group":"apis/networking.k8s.io","version":"v1","resource":"ingresses","namespace":"apps"}}}`),
			},
			want:    0,
			wantErr: true,
		},
		{
			name: "GlobalContextEntry enabled, neither KubernetesResource nor APICall present",
			args: args{
				opts: ValidationOptions{
					Enabled: true,
				},
				resource: []byte(`{"apiVersion":"kyverno.io/v2alpha1","kind":"GlobalContextEntry","metadata":{"name":"ingress"},"spec":{}}`),
			},
			want:    0,
			wantErr: true,
		},
		{
			name: "GlobalContextEntry enabled.",
			args: args{
				opts: ValidationOptions{
					Enabled: true,
				},
				resource: []byte(`{"apiVersion":"kyverno.io/v2alpha1","kind":"GlobalContextEntry","metadata":{"name":"ingress"},"spec":{"apiCall":{"service":{"url":"https://svc.kyverno/example","caBundle":"-----BEGIN CERTIFICATE-----\n-----REDACTED-----\n-----END CERTIFICATE-----"},"refreshInterval":"10ns"}}}`),
			},
			want:    0,
			wantErr: false,
		},
	}
	for _, c := range tc {
		t.Run(c.name, func(t *testing.T) {
			gctx, err := admissionutils.UnmarshalGlobalContextEntry(c.args.resource)
			assert.NilError(t, err)
			warnings, err := Validate(context.Background(), logging.GlobalLogger(), gctx, c.args.opts)
			if c.wantErr {
				assert.Assert(t, err != nil)
			} else {
				assert.NilError(t, err)
			}
			assert.Assert(t, len(warnings) == c.want)
		})
	}
}
