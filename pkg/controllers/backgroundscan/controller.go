package scanner

import (
	"context"

	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	scanScheduleKey = ".spec.scanSchedule"
)

// BackgroundScanReconciler reconciles a Kyverno policies
type BackgroundScanReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *BackgroundScanReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	var vpol kyvernov2alpha1.ValidatingPolicy
	if err := r.Get(ctx, req.NamespacedName, &vpol); err != nil {
		log.Error(err, "unable to fetch validation policy")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var vpolList kyvernov2alpha1.ValidatingPolicyList
	if err := r.List(ctx, &vpolList, client.InNamespace(req.Namespace), client.MatchingFields{scanScheduleKey: vpol.Spec.ScanSchedule}); err != nil {
		log.Error(err, "unable to list validating policies")
		return ctrl.Result{}, err
	}

	var filteredList []kyvernov2alpha1.ValidatingPolicy
	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackgroundScanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &kyvernov2alpha1.ValidatingPolicy{}, scanScheduleKey, func(rawObj client.Object) []string {
		vpol := rawObj.(*kyvernov2alpha1.ValidatingPolicy)
		scanSchedule := vpol.Spec.ScanSchedule

		// ...and if so, return it
		return []string{scanSchedule}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&kyvernov2alpha1.ValidatingPolicy{}).
		Named("scanner").
		Complete(r)
}
