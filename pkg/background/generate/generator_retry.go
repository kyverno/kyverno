package generate

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/background/generate/provenance"
	kyvernov2client "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

const pendingProvenanceAnnotation = "generate.kyverno.io/pending-provenance"

// A receipt retains the final stamp produced from an independently authorized
// server response. UpdateRequest metadata is not itself trusted: recovery must
// verify this MAC, its saved labels, and the live target before using the stamp.
type generationReceipt struct {
	Target kyvernov1.ResourceSpec `json:"target"`
	Labels map[string]string      `json:"labels"`
	Stamp  string                 `json:"stamp"`
}

type generationRetry struct {
	requests  kyvernov2client.UpdateRequestInterface
	name      string
	uid       types.UID
	policyUID types.UID
	receipts  []generationReceipt
	loaded    bool
}

func (r *generationRetry) read(ctx context.Context) (*kyvernov2.UpdateRequest, []generationReceipt, error) {
	ur, err := r.requests.Get(ctx, r.name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}
	if ur.UID != r.uid {
		return nil, nil, fmt.Errorf("update request %s was replaced", r.name)
	}
	if uid := ur.Annotations[provenance.PolicyUIDAnnotation]; uid != "" && uid != string(r.policyUID) {
		return nil, nil, fmt.Errorf("update request %s refers to a different policy", r.name)
	}
	var receipts []generationReceipt
	if raw := ur.Annotations[pendingProvenanceAnnotation]; raw != "" {
		if err := json.Unmarshal([]byte(raw), &receipts); err != nil {
			return nil, nil, fmt.Errorf("decode generated resource receipts: %w", err)
		}
	}
	return ur, receipts, nil
}

// Cache only within this reconciliation batch. A first live read avoids losing
// a receipt when the informer has not observed a previous failed attempt yet.
func (r *generationRetry) load(ctx context.Context) ([]generationReceipt, error) {
	if !r.loaded {
		_, receipts, err := r.read(ctx)
		if err != nil {
			return nil, err
		}
		r.receipts, r.loaded = receipts, true
	}
	return r.receipts, nil
}

func (r *generationRetry) save(ctx context.Context, receipt generationReceipt) error {
	return r.modify(ctx, func(receipts []generationReceipt) []generationReceipt {
		for _, existing := range receipts {
			if sameGenerationReceipt(existing, receipt) {
				return receipts
			}
		}
		return append(receipts, receipt)
	})
}

func (r *generationRetry) remove(ctx context.Context, receipt generationReceipt) error {
	return r.modify(ctx, func(receipts []generationReceipt) []generationReceipt {
		return slices.DeleteFunc(receipts, func(existing generationReceipt) bool {
			return sameGenerationReceipt(existing, receipt)
		})
	})
}

func (r *generationRetry) modify(ctx context.Context, change func([]generationReceipt) []generationReceipt) error {
	// Fresh reads preserve receipts from other rules/foreach elements and avoid
	// losing them when status updates or another worker change the request.
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		ur, receipts, err := r.read(ctx)
		if err != nil {
			return err
		}
		receipts = change(receipts)
		annotations := ur.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		previous := annotations[pendingProvenanceAnnotation]
		delete(annotations, pendingProvenanceAnnotation)
		if len(receipts) != 0 {
			encoded, err := json.Marshal(receipts)
			if err != nil {
				return err
			}
			annotations[pendingProvenanceAnnotation] = string(encoded)
		}
		if previous == annotations[pendingProvenanceAnnotation] {
			r.receipts, r.loaded = receipts, true
			return nil
		}
		ur.SetAnnotations(annotations)
		_, err = r.requests.Update(ctx, ur, metav1.UpdateOptions{})
		if err == nil {
			r.receipts, r.loaded = receipts, true
		}
		return err
	})
	if err != nil {
		r.loaded = false
	}
	return err
}

func sameGenerationReceipt(left, right generationReceipt) bool {
	return left.Target == right.Target && left.Stamp == right.Stamp && maps.Equal(left.Labels, right.Labels)
}

func sameGenerationTarget(left, right kyvernov1.ResourceSpec) bool {
	leftGV, leftErr := schema.ParseGroupVersion(left.APIVersion)
	rightGV, rightErr := schema.ParseGroupVersion(right.APIVersion)
	return leftErr == nil && rightErr == nil && leftGV.Group == rightGV.Group &&
		left.Kind == right.Kind && left.Namespace == right.Namespace && left.Name == right.Name
}

func (g *generator) resumeProvenance(ctx context.Context, target kyvernov1.ResourceSpec) (*kyvernov1.ResourceSpec, error) {
	if g.provenance == nil || g.pending == nil {
		return nil, nil
	}
	receipts, err := g.pending.load(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: read generated resource receipts: %w", errGeneratedProvenance, err)
	}
	for _, receipt := range receipts {
		// Omitted versions and namespaces are resolved by the existing dynamic
		// client. They are candidates only; the returned live identity must match
		// the authenticated receipt exactly before any patch is sent.
		candidate := target
		if candidate.APIVersion == "" {
			candidate.APIVersion = receipt.Target.APIVersion
		}
		if candidate.Namespace == "" && receipt.Target.Namespace == "default" {
			candidate.Namespace = "default"
		}
		if !sameGenerationTarget(receipt.Target, candidate) {
			continue
		}
		if !g.matchesGenerationContext(receipt.Labels) {
			continue
		}
		// Authenticate the saved snapshot, never a stamp derived from the live
		// resource or from unauthenticated GeneratedResources status entries.
		saved := &unstructured.Unstructured{}
		saved.SetAPIVersion(receipt.Target.APIVersion)
		saved.SetKind(receipt.Target.Kind)
		saved.SetNamespace(receipt.Target.Namespace)
		saved.SetName(receipt.Target.Name)
		saved.SetUID(receipt.Target.UID)
		saved.SetLabels(receipt.Labels)
		saved.SetAnnotations(map[string]string{provenance.Annotation: receipt.Stamp})
		valid, err := g.provenance.Verify(ctx, g.policy, saved)
		if err != nil || !valid {
			return nil, fmt.Errorf("%w: invalid generated resource receipt: %v", errGeneratedProvenance, err)
		}
		live, err := g.client.GetResource(ctx, target.APIVersion, target.Kind, target.Namespace, target.Name)
		if apierrors.IsNotFound(err) {
			if err := g.pending.remove(ctx, receipt); err != nil {
				return nil, fmt.Errorf("%w: retire missing generated resource receipt: %w", errGeneratedProvenance, err)
			}
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("%w: read pending generated resource: %w", errGeneratedProvenance, err)
		}
		if !sameGenerationTarget(receipt.Target, common.ResourceSpecFromUnstructured(*live)) || live.GetUID() != receipt.Target.UID || live.GetResourceVersion() == "" || !maps.Equal(generateRoutingLabels(live.GetLabels()), receipt.Labels) {
			return nil, fmt.Errorf("%w: pending generated resource identity or routing labels changed", errGeneratedProvenance)
		}
		if err := g.patchProvenance(ctx, live, receipt.Stamp); err != nil {
			return nil, err
		}
		if err := g.pending.remove(ctx, receipt); err != nil {
			return nil, fmt.Errorf("%w: retire generated resource receipt: %w", errGeneratedProvenance, err)
		}
		return &receipt.Target, nil
	}
	return nil, nil
}

func (g *generator) matchesGenerationContext(labels map[string]string) bool {
	expected := &unstructured.Unstructured{}
	common.ManageLabels(expected, g.trigger, g.policy, g.rule.Name)
	for key, value := range expected.GetLabels() {
		if labels[key] != value {
			return false
		}
	}
	return true
}

// An empty cloneList/foreach result or a now-skipped rule must not complete the
// request and discard receipts for targets that still need their annotation.
// Keep the existing bounded failure/retry behavior until an authorized rule
// evaluation selects those targets again; never recover from labels alone.
func (g *generator) checkPendingProvenance(ctx context.Context) error {
	if g.provenance == nil || g.pending == nil {
		return nil
	}
	receipts, err := g.pending.load(ctx)
	if err != nil {
		return fmt.Errorf("%w: read generated resource receipts: %w", errGeneratedProvenance, err)
	}
	for _, receipt := range receipts {
		if g.matchesGenerationContext(receipt.Labels) {
			return fmt.Errorf("%w: generated resource %s still has a pending receipt", errGeneratedProvenance, receipt.Target.String())
		}
	}
	return nil
}
