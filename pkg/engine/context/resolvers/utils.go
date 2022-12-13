package resolvers

import (
	"errors"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

func GetCacheSelector() (labels.Selector, error) {
	selector := labels.Everything()
	requirement, err := labels.NewRequirement(LabelCacheKey, selection.Exists, nil)
	if err != nil {
		return nil, err
	}
	return selector.Add(*requirement), err
}

func GetCacheInformerFactory(client kubernetes.Interface, resyncPeriod time.Duration) (kubeinformers.SharedInformerFactory, error) {
	if client == nil {
		return nil, errors.New("client cannot be nil")
	}
	selector, err := GetCacheSelector()
	if err != nil {
		return nil, err
	}
	return kubeinformers.NewSharedInformerFactoryWithOptions(
		client,
		resyncPeriod,
		kubeinformers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.LabelSelector = selector.String()
		}),
	), nil
}
