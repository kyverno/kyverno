package policyreport

import (
	"context"
	"sync"
	"time"

	"github.com/go-logr/logr"
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/toggle"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type item struct {
	crcr *kyvernov1alpha2.ClusterReportChangeRequest
	rcr  *kyvernov1alpha2.ReportChangeRequest
}

// creator is an interface that buffers report change requests
// merges and creates requests every tickerInterval
type creator interface {
	add(*kyvernov1alpha2.ClusterReportChangeRequest, *kyvernov1alpha2.ReportChangeRequest)
	run(stopChan <-chan struct{})
}

type changeRequestCreator struct {
	client         versioned.Interface
	mutex          *sync.RWMutex
	queue          []item
	tickerInterval time.Duration
	log            logr.Logger
}

func newChangeRequestCreator(client versioned.Interface, tickerInterval time.Duration, log logr.Logger) creator {
	return &changeRequestCreator{
		client:         client,
		mutex:          &sync.RWMutex{},
		queue:          []item{},
		tickerInterval: tickerInterval,
		log:            log,
	}
}

func (c *changeRequestCreator) add(crcr *kyvernov1alpha2.ClusterReportChangeRequest, rcr *kyvernov1alpha2.ReportChangeRequest) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.queue = append(c.queue, item{crcr, rcr})
}

func (c *changeRequestCreator) create(crcr *kyvernov1alpha2.ClusterReportChangeRequest, rcr *kyvernov1alpha2.ReportChangeRequest) error {
	if rcr != nil {
		_, err := c.client.KyvernoV1alpha2().ReportChangeRequests(config.KyvernoNamespace()).Create(context.TODO(), rcr, metav1.CreateOptions{})
		return err
	}
	if crcr != nil {
		_, err := c.client.KyvernoV1alpha2().ClusterReportChangeRequests().Create(context.TODO(), crcr, metav1.CreateOptions{})
		return err
	}
	return nil
}

func (c *changeRequestCreator) run(stopChan <-chan struct{}) {
	ticker := time.NewTicker(c.tickerInterval)
	defer ticker.Stop()
	if toggle.SplitPolicyReport.Enabled() {
		err := CleanupPolicyReport(c.client)
		if err != nil {
			c.log.Error(err, "failed to delete old reports")
		}
	}
	for {
		select {
		case <-ticker.C:
			var crcrs []*kyvernov1alpha2.ClusterReportChangeRequest
			var rcrs []*kyvernov1alpha2.ReportChangeRequest
			var size int
			if toggle.SplitPolicyReport.Enabled() {
				crcrs, rcrs, size = c.mergeRequestsPerPolicy()
			} else {
				crcrs, rcrs, size = c.mergeRequests()
			}
			for _, request := range crcrs {
				if err := c.create(request, nil); err != nil {
					c.log.Error(err, "failed to create report change request", "req", request)
				}
			}
			for _, request := range rcrs {
				if err := c.create(nil, request); err != nil {
					c.log.Error(err, "failed to create report change request", "req", request)
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
	c.queue = c.queue[size:]
}

// mergeRequests merges all current cached requests
// it blocks writing to the cache
func (c *changeRequestCreator) mergeRequests() ([]*kyvernov1alpha2.ClusterReportChangeRequest, []*kyvernov1alpha2.ReportChangeRequest, int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	var crcrs []*kyvernov1alpha2.ClusterReportChangeRequest
	var rcrs []*kyvernov1alpha2.ReportChangeRequest
	var mergedCRCR *kyvernov1alpha2.ClusterReportChangeRequest
	mergedRCRs := map[string]*kyvernov1alpha2.ReportChangeRequest{}
	size := len(c.queue)

	for _, item := range c.queue {
		if item.crcr != nil {
			// if it is a delete request
			if isDeleteRequest(item.crcr) {
				// if we accumulated something in the merged resource, add it and reset
				if mergedCRCR != nil {
					crcrs = append(crcrs, mergedCRCR)
					mergedCRCR = nil
				}
				// add the delete request
				crcrs = append(crcrs, item.crcr)
			} else {
				// if there is no accumulator yet, set it to the new item
				if mergedCRCR == nil {
					mergedCRCR = item.crcr
				} else {
					// try to merge the new item item with the accumulator, if it fails add the accumulator and reset it to the current item
					if ok := mergeCRCR(mergedCRCR, item.crcr); !ok {
						crcrs = append(crcrs, mergedCRCR)
						mergedCRCR = item.crcr
					}
				}
			}
		}
		if item.rcr != nil {
			resourceNS := item.rcr.GetLabels()[ResourceLabelNamespace]
			rcr := mergedRCRs[resourceNS]
			// if it is a delete request
			if isDeleteRequest(item.rcr) {
				// if we accumulated something in the merged resource, add it and reset
				if rcr != nil {
					rcrs = append(rcrs, rcr)
					rcr = nil
				}
				// add the delete request
				rcrs = append(rcrs, item.rcr)
			} else {
				// if there is no accumulator yet, set it to the new item
				if rcr == nil {
					rcr = item.rcr
				} else {
					// try to merge the new item item with the accumulator, if it fails add the accumulator and reset it to the current item
					if ok := mergeRCR(rcr, item.rcr); !ok {
						rcrs = append(rcrs, rcr)
						rcr = item.rcr
					}
				}
			}
			mergedRCRs[resourceNS] = rcr
		}
	}
	// last accumulators pass
	if mergedCRCR != nil {
		crcrs = append(crcrs, mergedCRCR)
	}
	for _, mergedNamespacedRCR := range mergedRCRs {
		if mergedNamespacedRCR != nil {
			rcrs = append(rcrs, mergedNamespacedRCR)
		}
	}
	return crcrs, rcrs, size
}

func (c *changeRequestCreator) mergeRequestsPerPolicy() ([]*kyvernov1alpha2.ClusterReportChangeRequest, []*kyvernov1alpha2.ReportChangeRequest, int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	var crcrs []*kyvernov1alpha2.ClusterReportChangeRequest
	var rcrs []*kyvernov1alpha2.ReportChangeRequest
	mergedCRCRs := map[string]*kyvernov1alpha2.ClusterReportChangeRequest{}
	mergedRCRs := map[string]*kyvernov1alpha2.ReportChangeRequest{}
	size := len(c.queue)

	for _, item := range c.queue {
		if item.crcr != nil {
			policy := item.crcr.GetLabels()[policyLabel]
			crcr := mergedCRCRs[policy]
			// if it is a delete request
			if isDeleteRequest(item.crcr) {
				// if we accumulated something in the merged resource, add it and reset
				if crcr != nil {
					crcrs = append(crcrs, crcr)
					crcr = nil
				}
				// add the delete request
				crcrs = append(crcrs, item.crcr)
			} else {
				// if there is no accumulator yet, set it to the new item
				if crcr == nil {
					crcr = item.crcr
				} else {
					// try to merge the new item item with the accumulator, if it fails add the accumulator and reset it to the current item
					if ok := mergeCRCR(crcr, item.crcr); !ok {
						crcrs = append(crcrs, crcr)
						crcr = item.crcr
					}
				}
			}
			mergedCRCRs[policy] = crcr
		}
		if item.rcr != nil {
			policy := item.rcr.GetLabels()[policyLabel]
			resourceNS := item.rcr.GetLabels()[ResourceLabelNamespace]
			key := resourceNS + "/" + policy
			rcr := mergedRCRs[key]
			// if it is a delete request
			if isDeleteRequest(item.rcr) {
				// if we accumulated something in the merged resource, add it and reset
				if rcr != nil {
					rcrs = append(rcrs, rcr)
					rcr = nil
				}
				// add the delete request
				rcrs = append(rcrs, item.rcr)
			} else {
				// if there is no accumulator yet, set it to the new item
				if rcr == nil {
					rcr = item.rcr
				} else {
					// try to merge the new item item with the accumulator, if it fails add the accumulator and reset it to the current item
					if ok := mergeRCR(rcr, item.rcr); !ok {
						rcrs = append(rcrs, rcr)
						rcr = item.rcr
					}
				}
			}
			mergedRCRs[key] = rcr
		}
	}
	for _, mergedPolicyCRCR := range mergedCRCRs {
		if mergedPolicyCRCR != nil {
			crcrs = append(crcrs, mergedPolicyCRCR)
		}
	}
	for _, mergedNamespacedRCR := range mergedRCRs {
		if mergedNamespacedRCR != nil {
			rcrs = append(rcrs, mergedNamespacedRCR)
		}
	}
	return crcrs, rcrs, size
}

func mergeRCR(dst, src *kyvernov1alpha2.ReportChangeRequest) bool {
	dstNS := dst.GetLabels()[ResourceLabelNamespace]
	srcNS := src.GetLabels()[ResourceLabelNamespace]
	if dstNS != srcNS {
		return false
	}
	dst.Results = append(dst.Results, src.Results...)
	dst.Summary = addSummary(dst.Summary, src.Summary)
	return true
}

func mergeCRCR(dst, src *kyvernov1alpha2.ClusterReportChangeRequest) bool {
	dstNS := dst.GetLabels()[ResourceLabelNamespace]
	srcNS := src.GetLabels()[ResourceLabelNamespace]
	if dstNS != srcNS {
		return false
	}
	dst.Results = append(dst.Results, src.Results...)
	dst.Summary = addSummary(dst.Summary, src.Summary)
	return true
}

func addSummary(dst, src policyreportv1alpha2.PolicyReportSummary) policyreportv1alpha2.PolicyReportSummary {
	return policyreportv1alpha2.PolicyReportSummary{
		Pass:  src.Pass + dst.Pass,
		Fail:  src.Fail + dst.Fail,
		Warn:  src.Warn + dst.Warn,
		Error: src.Error + dst.Error,
		Skip:  src.Skip + dst.Skip,
	}
}

func isDeleteRequest(request metav1.Object) bool {
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
