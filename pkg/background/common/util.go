package common

import (
	"context"
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var DefaultRetry = wait.Backoff{
	Steps:    15,
	Duration: 100 * time.Millisecond,
	Factor:   1.0,
	Jitter:   0.1,
}

func Update(client versioned.Interface, urLister kyvernov1beta1listers.UpdateRequestNamespaceLister, name string, mutator func(*kyvernov1beta1.UpdateRequest)) (*kyvernov1beta1.UpdateRequest, error) {
	var ur *kyvernov1beta1.UpdateRequest
	err := retry.RetryOnConflict(DefaultRetry, func() error {
		ur, err := urLister.Get(name)
		if err != nil {
			log.Log.Error(err, "[ATTEMPT] failed to fetch update request", "name", name)
			return err
		}
		ur = ur.DeepCopy()
		mutator(ur)
		_, err = client.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Update(context.TODO(), ur, metav1.UpdateOptions{})
		if err != nil {
			log.Log.Error(err, "[ATTEMPT] failed to update update request", "name", name)
		}
		return err
	})
	if err != nil {
		log.Log.Error(err, "failed to update update request", "name", name)
	} else {
		log.Log.V(3).Info("updated update request", "name", name, "status")
	}
	return ur, err
}

func UpdateStatus(client versioned.Interface, urLister kyvernov1beta1listers.UpdateRequestNamespaceLister, name string, state kyvernov1beta1.UpdateRequestState, message string, genResources []kyvernov1.ResourceSpec) (*kyvernov1beta1.UpdateRequest, error) {
	var ur *kyvernov1beta1.UpdateRequest
	err := retry.RetryOnConflict(DefaultRetry, func() error {
		ur, err := urLister.Get(name)
		if err != nil {
			log.Log.Error(err, "[ATTEMPT] failed to fetch update request", "name", name)
			return err
		}
		ur = ur.DeepCopy()
		ur.Status.State = state
		ur.Status.Message = message
		if genResources != nil {
			ur.Status.GeneratedResources = genResources
		}
		_, err = client.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).UpdateStatus(context.TODO(), ur, metav1.UpdateOptions{})
		if err != nil {
			log.Log.Error(err, "[ATTEMPT] failed to update update request status", "name", name)
			return err
		}
		return err
	})
	if err != nil {
		log.Log.Error(err, "failed to update update request status", "name", name)
	} else {
		log.Log.V(3).Info("updated update request status", "name", name, "status", string(state))
	}
	return ur, err
}
