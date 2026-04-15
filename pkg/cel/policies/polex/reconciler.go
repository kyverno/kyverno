package polex

import (
	"context"
	"maps"
	"sync"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/logging"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type reconciler struct {
	client     client.Client
	compiler   Compiler
	lock       *sync.RWMutex
	exceptions map[string]Exception
}

func newReconciler(
	compiler Compiler,
	client client.Client,
) *reconciler {
	return &reconciler{
		client:     client,
		compiler:   compiler,
		lock:       &sync.RWMutex{},
		exceptions: map[string]Exception{},
	}
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var exception policiesv1beta1.PolicyException
	err := r.client.Get(ctx, req.NamespacedName, &exception)
	if err != nil {
		if errors.IsNotFound(err) {
			r.lock.Lock()
			defer r.lock.Unlock()
			delete(r.exceptions, req.NamespacedName.String())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	compiled, errs := r.compiler.Compile(exception)
	if len(errs) > 0 {
		logging.V(4).Info("failed to compile exception", "name", exception.GetName(), "namespace", exception.GetNamespace(), "errors", errs)
		return ctrl.Result{}, nil
	}
	r.lock.Lock()
	defer r.lock.Unlock()
	r.exceptions[req.NamespacedName.String()] = *compiled
	return ctrl.Result{}, nil
}

func (r *reconciler) Fetch(ctx context.Context) ([]Exception, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()
	exceptions := make([]Exception, 0, len(r.exceptions))
	for value := range maps.Values(r.exceptions) {
		exceptions = append(exceptions, value)
	}
	return exceptions, nil
}
