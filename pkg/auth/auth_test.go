package auth

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/stretchr/testify/assert"
)

func TestNewCanI(t *testing.T) {
	type args struct {
		client    dclient.Interface
		kind      string
		namespace string
		verb      string
	}
	tests := []struct {
		name string
		args args
	}{{
		name: "deployments",
		args: args{
			client:    dclient.NewEmptyFakeClient(),
			kind:      "Deployment",
			namespace: "default",
			verb:      "test",
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewCanI(tt.args.client, tt.args.kind, tt.args.namespace, tt.args.verb)
			assert.NotNil(t, got)
		})
	}
}

func TestCanIOptions_RunAccessCheck(t *testing.T) {
	type fields struct {
		namespace string
		verb      string
		kind      string
		client    dclient.Interface
	}
	tests := []struct {
		name    string
		fields  fields
		want    bool
		wantErr bool
	}{{
		name: "deployments",
		fields: fields{
			client:    dclient.NewEmptyFakeClient(),
			kind:      "Deployment",
			namespace: "default",
			verb:      "test",
		},
		want:    false,
		wantErr: false,
	}, {
		name: "unknown",
		fields: fields{
			client:    dclient.NewEmptyFakeClient(),
			kind:      "Unknown",
			namespace: "default",
			verb:      "test",
		},
		want:    false,
		wantErr: true,
	}, {
		name: "v2 pods",
		fields: fields{
			client:    dclient.NewEmptyFakeClient(),
			kind:      "v2/Pod",
			namespace: "default",
			verb:      "test",
		},
		want:    false,
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := NewCanI(tt.fields.client, tt.fields.kind, tt.fields.namespace, tt.fields.verb)
			got, err := o.RunAccessCheck()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
