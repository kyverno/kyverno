package k8sresource

// import (
// 	"fmt"
// 	"io"
// 	"sync"
// 	"time"

// 	"github.com/go-logr/logr"
// 	apierrors "k8s.io/apimachinery/pkg/api/errors"
// 	"k8s.io/apimachinery/pkg/runtime/schema"
// 	k8scache "k8s.io/client-go/tools/cache"
// )

// // The WatchErrorHandler is called whenever ListAndWatch drops the
// // connection with an error.
// // By default, the error is reported in logs but the end user of resource
// // cache will not know what error has been reported. This custom WatchErrorHandler
// // will store the error received.
// // Since the watch handler is only called when there is an error, we
// // have to clear the error after sometime ourselves. This handler uses an
// // ephemeral store which will clear the data after sometime. We have set the clear time
// // at slightly above the informer resync period.
// type WatchErrorHandler struct {
// 	logger    logr.Logger
// 	data      *EphemeralData
// 	resource  schema.GroupVersionResource
// 	namespace string
// }

// func NewWatchErrorHandler(logger logr.Logger, resource schema.GroupVersionResource, namespace string) *WatchErrorHandler {
// 	return &WatchErrorHandler{
// 		logger:    logger,
// 		data:      NewEphemeralData(resyncPeriod),
// 		resource:  resource,
// 		namespace: namespace,
// 	}
// }

// func (w *WatchErrorHandler) WatchErrorHandlerFunction() k8scache.WatchErrorHandler {
// 	fn := func(r *k8scache.Reflector, err error) {
// 		switch {
// 		case apierrors.IsResourceExpired(err) || apierrors.IsGone(err):
// 			w.logger.V(4).Info("watch closed with error", "resource", w.resource.String(), "error", err)
// 		case err == io.EOF:
// 			// watch closed normally
// 		case err == io.ErrUnexpectedEOF:
// 			w.logger.V(4).Info("Watch closed with unexpected EOF:", "resource", w.resource.String(), "error", err)
// 		default:
// 			err := fmt.Errorf("Failed to watch %v, Namespace=%v %v", w.resource.String(), w.namespace, err)
// 			w.logger.Error(err, "error from watch error handler ")
// 			w.data.Set(err)
// 		}
// 	}
// 	return fn
// }

// func (w *WatchErrorHandler) Error() error {
// 	data := w.data.Get()
// 	if data != nil {
// 		w.logger.V(2).Info("Error from watcher:", data)
// 	}
// 	return data
// }

// // EphemeralData stores an error which expires after a duration.
// type EphemeralData struct {
// 	sync.Mutex
// 	data        error
// 	clearAfter  time.Duration
// 	lastUpdated time.Time
// }

// func NewEphemeralData(clearAfter time.Duration) *EphemeralData {
// 	return &EphemeralData{
// 		clearAfter: clearAfter,
// 	}
// }

// func (e *EphemeralData) Set(data error) {
// 	e.Lock()
// 	defer e.Unlock()

// 	e.data = data
// 	e.lastUpdated = time.Now()
// }

// func (e *EphemeralData) Get() error {
// 	e.Lock()
// 	defer e.Unlock()

// 	if time.Since(e.lastUpdated) > e.clearAfter {
// 		e.data = nil
// 	}

// 	return e.data
// }
