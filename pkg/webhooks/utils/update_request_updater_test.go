package utils

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	kyvernoinformers "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	kyvernov1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestNewUpdateRequestUpdater(t *testing.T) {
	type args struct {
		client versioned.Interface
		lister kyvernov1beta1listers.UpdateRequestNamespaceLister
	}
	tests := []struct {
		name string
		args args
		want UpdateRequestUpdater
	}{{
		name: "nil",
		args: args{nil, nil},
		want: &updateRequestUpdater{nil, nil},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewUpdateRequestUpdater(tt.args.client, tt.args.lister)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_updateRequestUpdater_updateAnnotation(t *testing.T) {
	type data struct {
		objects []runtime.Object
	}
	tests := []struct {
		name    string
		data    data
		urName  string
		updated bool
	}{{
		name: "success",
		data: data{
			[]runtime.Object{
				&kyvernov1beta1.UpdateRequest{
					ObjectMeta: v1.ObjectMeta{
						Name:      "test",
						Namespace: config.KyvernoNamespace(),
					},
				},
			},
		},
		urName:  "test",
		updated: true,
	}, {
		name: "not found",
		data: data{
			[]runtime.Object{
				&kyvernov1beta1.UpdateRequest{
					ObjectMeta: v1.ObjectMeta{
						Name:      "dummy",
						Namespace: config.KyvernoNamespace(),
					},
				},
			},
		},
		urName:  "dummy",
		updated: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			done := ctx.Done()
			t.Cleanup(cancel)
			client := fake.NewSimpleClientset(tt.data.objects...)
			kyvernoInformers := kyvernoinformers.NewSharedInformerFactory(client, 0)
			lister := kyvernoInformers.Kyverno().V1beta1().UpdateRequests().Lister().UpdateRequests(config.KyvernoNamespace())
			kyvernoInformers.Start(done)
			kyvernoInformers.WaitForCacheSync(done)
			h := &updateRequestUpdater{
				client: client,
				lister: lister,
			}
			h.updateAnnotation(logr.Discard(), "test")
			ur, err := client.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Get(ctx, tt.urName, v1.GetOptions{})
			assert.NoError(t, err)
			assert.NotNil(t, ur)
			if tt.updated {
				annotations := ur.GetAnnotations()
				assert.NotNil(t, annotations)
				assert.NotNil(t, annotations["generate.kyverno.io/updation-time"])
			} else {
				annotations := ur.GetAnnotations()
				assert.Nil(t, annotations)
			}
		})
	}
}

func Test_updateRequestUpdater_setPendingStatus(t *testing.T) {
	type data struct {
		objects []runtime.Object
	}
	tests := []struct {
		name    string
		data    data
		urName  string
		updated bool
	}{{
		name: "success",
		data: data{
			[]runtime.Object{
				&kyvernov1beta1.UpdateRequest{
					ObjectMeta: v1.ObjectMeta{
						Name:      "test",
						Namespace: config.KyvernoNamespace(),
					},
				},
			},
		},
		urName:  "test",
		updated: true,
	}, {
		name: "not found",
		data: data{
			[]runtime.Object{
				&kyvernov1beta1.UpdateRequest{
					ObjectMeta: v1.ObjectMeta{
						Name:      "dummy",
						Namespace: config.KyvernoNamespace(),
					},
				},
			},
		},
		urName:  "dummy",
		updated: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			done := ctx.Done()
			t.Cleanup(cancel)
			client := fake.NewSimpleClientset(tt.data.objects...)
			kyvernoInformers := kyvernoinformers.NewSharedInformerFactory(client, 0)
			lister := kyvernoInformers.Kyverno().V1beta1().UpdateRequests().Lister().UpdateRequests(config.KyvernoNamespace())
			kyvernoInformers.Start(done)
			kyvernoInformers.WaitForCacheSync(done)
			h := &updateRequestUpdater{
				client: client,
				lister: lister,
			}
			h.setPendingStatus(logr.Discard(), "test")
			ur, err := client.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Get(ctx, tt.urName, v1.GetOptions{})
			assert.NoError(t, err)
			assert.NotNil(t, ur)
			if tt.updated {
				assert.Equal(t, kyvernov1beta1.Pending, ur.Status.State)
			} else {
				assert.NotEqual(t, kyvernov1beta1.Pending, ur.Status.State)
			}
		})
	}
}
