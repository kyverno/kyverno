package controller

import (
	"context"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/metrics"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/metric"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type reconcileFunc func(ctx context.Context, logger logr.Logger, key string, namespace string, name string) error

type controllerMetrics struct {
	controllerName string
	reconcileTotal sdkmetric.Int64Counter
	requeueTotal   sdkmetric.Int64Counter
	queueDropTotal sdkmetric.Int64Counter
}

func newControllerMetrics(logger logr.Logger, controllerName string) *controllerMetrics {
	meter := otel.GetMeterProvider().Meter(metrics.MeterName)
	reconcileTotal, err := meter.Int64Counter(
		"kyverno_controller_reconcile",
		sdkmetric.WithDescription("can be used to track number of reconciliation cycles"))
	if err != nil {
		logger.Error(err, "Failed to create instrument, kyverno_controller_reconcile_total")
	}
	requeueTotal, err := meter.Int64Counter(
		"kyverno_controller_requeue",
		sdkmetric.WithDescription("can be used to track number of reconciliation errors"))
	if err != nil {
		logger.Error(err, "Failed to create instrument, kyverno_controller_requeue_total")
	}
	queueDropTotal, err := meter.Int64Counter(
		"kyverno_controller_drop",
		sdkmetric.WithDescription("can be used to track number of queue drops"))
	if err != nil {
		logger.Error(err, "Failed to create instrument, kyverno_controller_drop_total")
	}
	return &controllerMetrics{
		controllerName: controllerName,
		reconcileTotal: reconcileTotal,
		requeueTotal:   requeueTotal,
		queueDropTotal: queueDropTotal,
	}
}

func Run(ctx context.Context, logger logr.Logger, controllerName string, period time.Duration, queue workqueue.RateLimitingInterface, n, maxRetries int, r reconcileFunc, routines ...func(context.Context, logr.Logger)) {
	logger.Info("starting ...")
	defer logger.Info("stopped")
	var wg sync.WaitGroup
	defer wg.Wait()
	defer runtime.HandleCrash()
	metric := newControllerMetrics(logger, controllerName)
	func() {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		defer queue.ShutDown()
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(logger logr.Logger) {
				logger.Info("starting worker")
				defer logger.Info("worker stopped")
				defer wg.Done()
				wait.UntilWithContext(ctx, func(ctx context.Context) { worker(ctx, logger, metric, queue, maxRetries, r) }, period)
			}(logger.WithName("worker").WithValues("id", i))
		}
		for i, routine := range routines {
			wg.Add(1)
			go func(logger logr.Logger, routine func(context.Context, logr.Logger)) {
				logger.Info("starting routine")
				defer logger.Info("routine stopped")
				defer wg.Done()
				routine(ctx, logger)
			}(logger.WithName("routine").WithValues("id", i), routine)
		}
		<-ctx.Done()
	}()
	logger.Info("waiting for workers to terminate ...")
}

func worker(ctx context.Context, logger logr.Logger, metric *controllerMetrics, queue workqueue.RateLimitingInterface, maxRetries int, r reconcileFunc) {
	for processNextWorkItem(ctx, logger, metric, queue, maxRetries, r) {
	}
}

func processNextWorkItem(ctx context.Context, logger logr.Logger, metric *controllerMetrics, queue workqueue.RateLimitingInterface, maxRetries int, r reconcileFunc) bool {
	if obj, quit := queue.Get(); !quit {
		defer queue.Done(obj)
		handleErr(ctx, logger, metric, queue, maxRetries, reconcile(ctx, logger, obj, r), obj)
		return true
	}
	return false
}

func handleErr(ctx context.Context, logger logr.Logger, metric *controllerMetrics, queue workqueue.RateLimitingInterface, maxRetries int, err error, obj interface{}) {
	if metric.reconcileTotal != nil {
		metric.reconcileTotal.Add(ctx, 1, sdkmetric.WithAttributes(attribute.String("controller_name", metric.controllerName)))
	}
	if err == nil {
		queue.Forget(obj)
	} else if errors.IsNotFound(err) {
		logger.Info("Dropping request from the queue", "obj", obj, "error", err.Error())
		queue.Forget(obj)
	} else if queue.NumRequeues(obj) < maxRetries {
		logger.Info("Retrying request", "obj", obj, "error", err.Error())
		queue.AddRateLimited(obj)
		if metric.requeueTotal != nil {
			metric.requeueTotal.Add(
				ctx,
				1,
				sdkmetric.WithAttributes(
					attribute.String("controller_name", metric.controllerName),
					attribute.Int("num_requeues", queue.NumRequeues(obj)),
				),
			)
		}
	} else {
		logger.Error(err, "Failed to process request", "obj", obj)
		queue.Forget(obj)
		if metric.queueDropTotal != nil {
			metric.queueDropTotal.Add(
				ctx,
				1,
				sdkmetric.WithAttributes(
					attribute.String("controller_name", metric.controllerName),
				),
			)
		}
	}
}

func reconcile(ctx context.Context, logger logr.Logger, obj interface{}, r reconcileFunc) error {
	start := time.Now()
	var k, ns, n string
	if key, ok := obj.(cache.ExplicitKey); ok {
		k = string(key)
	} else {
		k = obj.(string)
		if namespace, name, err := cache.SplitMetaNamespaceKey(k); err != nil {
			return err
		} else {
			ns, n = namespace, name
		}
	}
	logger = logger.WithValues("key", k, "namespace", ns, "name", n)
	logger.V(4).Info("reconciling ...")
	defer func(start time.Time) {
		logger.V(4).Info("done", "duration", time.Since(start).String())
	}(start)
	return r(ctx, logger, k, ns, n)
}
