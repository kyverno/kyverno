package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	authorizationv1client "k8s.io/client-go/kubernetes/typed/authorization/v1"
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
			got := NewCanI(tt.args.client.Discovery(), tt.args.client.GetKubeClient().AuthorizationV1().SubjectAccessReviews(), tt.args.kind, tt.args.namespace, tt.args.verb, "", "admin")
			assert.NotNil(t, got)
		})
	}
}

type discovery struct{}

func (d *discovery) GetGVRFromGVK(schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	return schema.GroupVersionResource{}, errors.New("dummy")
}

func TestCanIOptions_DiscoveryError(t *testing.T) {
	type fields struct {
		namespace string
		verb      string
		kind      string
		discovery Discovery
	}
	tests := []struct {
		name    string
		fields  fields
		want    bool
		wantErr bool
	}{{
		name: "deployments",
		fields: fields{
			discovery: &discovery{},
			kind:      "Deployment",
			namespace: "default",
			verb:      "test",
		},
		want:    false,
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := NewCanI(tt.fields.discovery, nil, tt.fields.kind, tt.fields.namespace, tt.fields.verb, "", "admin")
			got, err := o.RunAccessCheck(context.TODO())
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

type sar struct{}

func (d *sar) Create(_ context.Context, _ *v1.SubjectAccessReview, _ metav1.CreateOptions) (*v1.SubjectAccessReview, error) {
	return nil, errors.New("dummy")
}

func TestCanIOptions_SsarError(t *testing.T) {
	type fields struct {
		namespace string
		verb      string
		kind      string
		discovery Discovery
		sarClient authorizationv1client.SubjectAccessReviewInterface
	}
	tests := []struct {
		name    string
		fields  fields
		want    bool
		wantErr bool
	}{{
		name: "deployments",
		fields: fields{
			discovery: dclient.NewEmptyFakeClient().Discovery(),
			sarClient: &sar{},
			kind:      "Deployment",
			namespace: "default",
			verb:      "test",
		},
		want:    false,
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := NewCanI(tt.fields.discovery, tt.fields.sarClient, tt.fields.kind, tt.fields.namespace, tt.fields.verb, "", "admin")
			got, err := o.RunAccessCheck(context.TODO())
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
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
			o := NewCanI(tt.fields.client.Discovery(), tt.fields.client.GetKubeClient().AuthorizationV1().SubjectAccessReviews(), tt.fields.kind, tt.fields.namespace, tt.fields.verb, "", "admin")
			got, err := o.RunAccessCheck(context.TODO())
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
