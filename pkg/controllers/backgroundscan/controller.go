package scanner

import (
	"context"
	"time"

	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/robfig/cron"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	// TODO: Add an interface that sends report data directly to reports aggregator
}

func (r *BackgroundScanReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	now := time.Now()

	var vpol kyvernov2alpha1.ValidatingPolicy
	if err := r.Get(ctx, req.NamespacedName, &vpol); err != nil {
		log.Error(err, "unable to fetch validation policy")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// TODO: Add a global flag for default scan schedule
	sched, err := cron.ParseStandard(vpol.Spec.ScanSchedule)
	if err != nil {
		log.Error(err, "unable to parse scan schedule")
		return ctrl.Result{}, nil
	}

	// the policy should be rescheduled at the next activation time after current time
	schedResult := reconcile.Result{RequeueAfter: sched.Next(now).Sub(now)}

	getLastScheduledTime := func(vpol *kyvernov2alpha1.ValidatingPolicy) time.Time {
		var lastSchedule time.Time
		if vpol.Status.LastScheduleTime != nil {
			lastSchedule = vpol.Status.LastScheduleTime.Time
		} else {
			lastSchedule = vpol.ObjectMeta.CreationTimestamp.Time
		}
		return lastSchedule

	}
	// If current time is before next scheduled time, that means the policy has been processed already
	if now.Before(sched.Next(getLastScheduledTime(&vpol))) {
		return schedResult, nil
	}

	// Fetch all policies with same cron schedule
	var vpolList kyvernov2alpha1.ValidatingPolicyList
	if err := r.List(ctx, &vpolList, client.InNamespace(req.Namespace), client.MatchingFields{scanScheduleKey: vpol.Spec.ScanSchedule}); err != nil {
		log.Error(err, "unable to list validating policies")
		return ctrl.Result{}, err
	}

	var filteredList []kyvernov2alpha1.ValidatingPolicy
	for _, v := range vpolList.Items {
		// If the next scheduled run of cron is in the past, run it
		if now.After(sched.Next(getLastScheduledTime(&v))) {
			filteredList = append(filteredList, v)
		}
	}
	// TODO: Reuse logic for resource fetching from existing background controller

	// TODO: Temporarily reuse logic for reports creation from background controller

	for _, v := range filteredList {
		v.Status.LastScheduleTime = &metav1.Time{Time: now}

		if err := r.Status().Update(ctx, &v); err != nil {
			log.Error(err, "unable to last scheduled time on policy")
		}
	}

	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackgroundScanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create an index for all validating polices with same cron schedule
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &kyvernov2alpha1.ValidatingPolicy{}, scanScheduleKey, func(rawObj client.Object) []string {
		vpol := rawObj.(*kyvernov2alpha1.ValidatingPolicy)
		scanSchedule := vpol.Spec.ScanSchedule

		return []string{scanSchedule}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&kyvernov2alpha1.ValidatingPolicy{}).
		Named("scanner").
		Complete(r)
}
