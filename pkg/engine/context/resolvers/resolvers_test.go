package resolvers

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/kyverno/kyverno/pkg/engine/api"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

const (
	namespace = "default"
	name      = "myconfigmap"
)

func newEmptyFakeClient() *kubefake.Clientset {
	return kubefake.NewSimpleClientset()
}

func createConfigMaps(ctx context.Context, client *kubefake.Clientset, addLabel bool) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string]string{"configmapkey": "key1"},
	}
	if addLabel {
		cm.ObjectMeta.Labels = map[string]string{kyverno.LabelCacheEnabled: "true"}
	}
	_, err := client.CoreV1().ConfigMaps(namespace).Create(
		ctx, cm, metav1.CreateOptions{})
	return err
}

func initialiseInformer(client *kubefake.Clientset) kubeinformers.SharedInformerFactory {
	selector, err := GetCacheSelector()
	if err != nil {
		return nil
	}
	labelOptions := kubeinformers.WithTweakListOptions(func(opts *metav1.ListOptions) {
		opts.LabelSelector = selector.String()
	})
	kubeResourceInformer := kubeinformers.NewSharedInformerFactoryWithOptions(client, 15*time.Minute, labelOptions)
	return kubeResourceInformer
}

func Test_InformerCacheSuccess(t *testing.T) {
	client := newEmptyFakeClient()
	ctx := context.TODO()
	err := createConfigMaps(ctx, client, true)
	assert.NilError(t, err, "error while creating configmap")
	informer := initialiseInformer(client)
	informerResolver, err := NewInformerBasedResolver(informer.Core().V1().ConfigMaps().Lister())
	assert.NilError(t, err)
	informer.Start(make(<-chan struct{}))
	time.Sleep(10 * time.Second)
	_, err = informerResolver.Get(ctx, namespace, name)
	assert.NilError(t, err, "informer didn't have expected configmap")
}

func Test_InformerCacheFailure(t *testing.T) {
	client := newEmptyFakeClient()
	ctx := context.TODO()
	err := createConfigMaps(ctx, client, false)
	assert.NilError(t, err, "error while creating configmap")
	informer := initialiseInformer(client)
	resolver, err := NewInformerBasedResolver(informer.Core().V1().ConfigMaps().Lister())
	assert.NilError(t, err)
	informer.Start(make(<-chan struct{}))
	time.Sleep(10 * time.Second)
	_, err = resolver.Get(ctx, namespace, name)
	assert.Equal(t, err.Error(), "configmap \"myconfigmap\" not found")
}

func Test_ClientBasedResolver(t *testing.T) {
	client := newEmptyFakeClient()
	ctx := context.TODO()
	err := createConfigMaps(ctx, client, false)
	assert.NilError(t, err, "error while creating configmap")
	resolver, err := NewClientBasedResolver(client)
	assert.NilError(t, err)
	_, err = resolver.Get(ctx, namespace, name)
	assert.NilError(t, err, "error while getting configmap from client")
}

func Test_ResolverChainWithExistingConfigMap(t *testing.T) {
	client := newEmptyFakeClient()
	informer := initialiseInformer(client)
	lister := informer.Core().V1().ConfigMaps().Lister()
	informerBasedResolver, err := NewInformerBasedResolver(lister)
	assert.NilError(t, err)
	clientBasedResolver, err := NewClientBasedResolver(client)
	assert.NilError(t, err)
	resolvers, err := api.NewNamespacedResourceResolver(informerBasedResolver, clientBasedResolver)
	assert.NilError(t, err)
	ctx := context.TODO()
	err = createConfigMaps(ctx, client, true)
	assert.NilError(t, err, "error while creating configmap")
	_, err = resolvers.Get(ctx, namespace, name)
	assert.NilError(t, err, "error while getting configmap")
}

func Test_ResolverChainWithNonExistingConfigMap(t *testing.T) {
	client := newEmptyFakeClient()
	informer := initialiseInformer(client)
	lister := informer.Core().V1().ConfigMaps().Lister()
	informerBasedResolver, err := NewInformerBasedResolver(lister)
	assert.NilError(t, err)
	clientBasedResolver, err := NewClientBasedResolver(client)
	assert.NilError(t, err)
	resolvers, err := api.NewNamespacedResourceResolver(informerBasedResolver, clientBasedResolver)
	assert.NilError(t, err)
	ctx := context.TODO()
	_, err = resolvers.Get(ctx, namespace, name)
	assert.Error(t, err, "configmaps \"myconfigmap\" not found")
}

func TestNewInformerBasedResolver(t *testing.T) {
	type args struct {
		lister corev1listers.ConfigMapLister
	}
	client := newEmptyFakeClient()
	informer := initialiseInformer(client)
	lister := informer.Core().V1().ConfigMaps().Lister()
	tests := []struct {
		name    string
		args    args
		want    api.ConfigmapResolver
		wantErr bool
	}{{
		name:    "nil shoud return an error",
		wantErr: true,
	}, {
		name: "not nil",
		args: args{lister},
		want: &informerBasedResolver{lister},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewInformerBasedResolver(tt.args.lister)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewInformerBasedResolver() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewInformerBasedResolver() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewClientBasedResolver(t *testing.T) {
	type args struct {
		client kubernetes.Interface
	}
	client := newEmptyFakeClient()
	tests := []struct {
		name    string
		args    args
		want    api.ConfigmapResolver
		wantErr bool
	}{{
		name:    "nil shoud return an error",
		wantErr: true,
	}, {
		name: "not nil",
		args: args{client},
		want: &clientBasedResolver{client},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewClientBasedResolver(tt.args.client)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClientBasedResolver() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewClientBasedResolver() = %v, want %v", got, tt.want)
			}
		})
	}
}
