package policyreport

import (
	"crypto/rand"
	"math/big"
	"reflect"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/patrickmn/go-cache"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// creator is an interface that buffers report change requests
// merges and creates requests every tickerInterval
type creator interface {
	add(request *unstructured.Unstructured)
	create(request *unstructured.Unstructured) error
	run(stopChan <-chan struct{})
}

type changeRequestCreator struct {
	dclient *dclient.Client

	// addCache preserves requests that are to be added to report
	RCRCache *cache.Cache

	CRCRCache *cache.Cache
	// removeCache preserves requests that are to be removed from report
	// removeCache *cache.Cache
	mutex sync.RWMutex
	queue []string

	tickerInterval time.Duration

	log logr.Logger
}

func newChangeRequestCreator(client *dclient.Client, tickerInterval time.Duration, log logr.Logger) creator {
	return &changeRequestCreator{
		dclient:        client,
		RCRCache:       cache.New(0, 24*time.Hour),
		CRCRCache:      cache.New(0, 24*time.Hour),
		queue:          []string{},
		tickerInterval: tickerInterval,
		log:            log,
	}
}

func (c *changeRequestCreator) add(request *unstructured.Unstructured) {
	uid, _ := rand.Int(rand.Reader, big.NewInt(100000))
	var err error

	switch request.GetKind() {
	case "ClusterReportChangeRequest":
		err = c.CRCRCache.Add(uid.String(), request, cache.NoExpiration)
		if err != nil {
			c.log.Error(err, "failed to add ClusterReportChangeRequest to cache")
		}
	case "ReportChangeRequest":
		err = c.RCRCache.Add(uid.String(), request, cache.NoExpiration)
		if err != nil {
			c.log.Error(err, "failed to add ReportChangeRequest to cache")
		}
	default:
		return
	}

	c.mutex.Lock()
	c.queue = append(c.queue, uid.String())
	c.mutex.Unlock()
}

func (c *changeRequestCreator) create(request *unstructured.Unstructured) error {
	ns := ""
	if request.GetKind() == "ReportChangeRequest" {
		ns = config.KyvernoNamespace
	}
	_, err := c.dclient.CreateResource(request.GetAPIVersion(), request.GetKind(), ns, request, false)
	return err
}

func (c *changeRequestCreator) run(stopChan <-chan struct{}) {
	ticker := time.NewTicker(c.tickerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			requests, size := c.mergeRequests()
			for _, request := range requests {
				if err := c.create(request); err != nil {
					c.log.Error(err, "failed to create report change request", "req", request.Object)
				}
			}

			c.cleanupQueue(size)
		case <-stopChan:
			return
		}
	}
}

func (c *changeRequestCreator) cleanupQueue(size int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for i := 0; i < size; i++ {
		uid := c.queue[i]
		c.CRCRCache.Delete(uid)
		c.RCRCache.Delete(uid)
	}

	c.queue = c.queue[size:]
}

// mergeRequests merges all current cached requests
// it blocks writing to the cache
func (c *changeRequestCreator) mergeRequests() (results []*unstructured.Unstructured, size int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	mergedCRCR := &unstructured.Unstructured{}
	mergedRCR := make(map[string]*unstructured.Unstructured)
	size = len(c.queue)

	for _, uid := range c.queue {
		if unstr, ok := c.CRCRCache.Get(uid); ok {
			if crcr, ok := unstr.(*unstructured.Unstructured); ok {
				if isDeleteRequest(crcr) {
					if !reflect.DeepEqual(mergedCRCR, &unstructured.Unstructured{}) {
						results = append(results, mergedCRCR)
						mergedCRCR = &unstructured.Unstructured{}
					}

					results = append(results, crcr)
				} else {
					if reflect.DeepEqual(mergedCRCR, &unstructured.Unstructured{}) {
						mergedCRCR = crcr
						continue
					}

					if ok := merge(mergedCRCR, crcr); !ok {
						results = append(results, mergedCRCR)
						mergedCRCR = crcr
					}
				}
			}
			continue
		}

		if unstr, ok := c.RCRCache.Get(uid); ok {
			if rcr, ok := unstr.(*unstructured.Unstructured); ok {
				resourceNS := rcr.GetLabels()[resourceLabelNamespace]
				mergedNamespacedRCR, ok := mergedRCR[resourceNS]
				if !ok {
					mergedNamespacedRCR = &unstructured.Unstructured{}
				}

				if isDeleteRequest(rcr) {
					if !reflect.DeepEqual(mergedNamespacedRCR, &unstructured.Unstructured{}) {
						results = append(results, mergedNamespacedRCR)
						mergedRCR[resourceNS] = &unstructured.Unstructured{}
					}

					results = append(results, rcr)
				} else {
					if reflect.DeepEqual(mergedNamespacedRCR, &unstructured.Unstructured{}) {
						mergedRCR[resourceNS] = rcr
						continue
					}

					if ok := merge(mergedNamespacedRCR, rcr); !ok {
						results = append(results, mergedNamespacedRCR)
						mergedRCR[resourceNS] = rcr
					} else {
						mergedRCR[resourceNS] = mergedNamespacedRCR
					}
				}
			}
		}
	}

	if !reflect.DeepEqual(mergedCRCR, &unstructured.Unstructured{}) {
		results = append(results, mergedCRCR)
	}

	for _, mergedNamespacedRCR := range mergedRCR {
		if !reflect.DeepEqual(mergedNamespacedRCR, &unstructured.Unstructured{}) {
			results = append(results, mergedNamespacedRCR)
		}
	}

	return
}

// merge merges elements from a source object into a
// destination object if they share the same namespace label
func merge(dst, src *unstructured.Unstructured) bool {
	dstNS := dst.GetLabels()[resourceLabelNamespace]
	srcNS := src.GetLabels()[resourceLabelNamespace]
	if dstNS != srcNS {
		return false
	}

	if dstResults, ok, _ := unstructured.NestedSlice(dst.UnstructuredContent(), "results"); ok {
		if srcResults, ok, _ := unstructured.NestedSlice(src.UnstructuredContent(), "results"); ok {
			dstResults = append(dstResults, srcResults...)

			if err := unstructured.SetNestedSlice(dst.UnstructuredContent(), dstResults, "results"); err == nil {
				err = addSummary(dst, src)
				return err == nil
			}
		}
	}
	return false
}

func addSummary(dst, src *unstructured.Unstructured) error {
	if dstSum, ok, _ := unstructured.NestedMap(dst.UnstructuredContent(), "summary"); ok {
		if srcSum, ok, _ := unstructured.NestedMap(src.UnstructuredContent(), "summary"); ok {
			for key, dstVal := range dstSum {
				if dstValInt, ok := dstVal.(int64); ok {
					if srcVal, ok := srcSum[key].(int64); ok {
						dstSum[key] = dstValInt + srcVal
					}
				}
			}
		}
		return unstructured.SetNestedMap(dst.UnstructuredContent(), dstSum, "summary")
	}
	return nil
}

func isDeleteRequest(request *unstructured.Unstructured) bool {
	deleteLabels := []string{deletedLabelPolicy, deletedLabelRule}
	labels := request.GetLabels()

	for _, l := range deleteLabels {
		if _, ok := labels[l]; ok {
			return true
		}
	}

	deleteAnnotations := []string{deletedAnnotationResourceName, deletedAnnotationResourceKind}
	annotations := request.GetAnnotations()
	for _, ann := range deleteAnnotations {
		if _, ok := annotations[ann]; ok {
			return true
		}
	}

	return false
}
