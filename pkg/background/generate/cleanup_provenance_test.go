package generate

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/background/generate/provenance"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type provenanceCleanupClient struct {
	dclient.Interface
	items        []unstructured.Unstructured
	deleted      []string
	beforeDelete func(*unstructured.Unstructured)
}

func (c *provenanceCleanupClient) ListResource(context.Context, string, string, string, *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	list := &unstructured.UnstructuredList{Items: c.items}
	return list.DeepCopy(), nil
}

func (c *provenanceCleanupClient) GetResource(_ context.Context, _, _, _, name string, _ ...string) (*unstructured.Unstructured, error) {
	for i := range c.items {
		if c.items[i].GetName() == name {
			return c.items[i].DeepCopy(), nil
		}
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, name)
}

func (c *provenanceCleanupClient) DeleteResource(_ context.Context, _, _, _, name string, _ bool, options metav1.DeleteOptions) error {
	for i := range c.items {
		resource := &c.items[i]
		if resource.GetName() != name {
			continue
		}
		if c.beforeDelete != nil {
			c.beforeDelete(resource)
		}
		if options.Preconditions == nil || options.Preconditions.UID == nil || *options.Preconditions.UID != resource.GetUID() ||
			(options.Preconditions.ResourceVersion != nil && *options.Preconditions.ResourceVersion != resource.GetResourceVersion()) {
			return apierrors.NewConflict(schema.GroupResource{Resource: "configmaps"}, name, fmt.Errorf("delete precondition failed"))
		}
		c.deleted = append(c.deleted, name)
		return nil
	}
	return apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, name)
}

func TestCleanupRequiresAuthenticatedRuleAndTrigger(t *testing.T) {
	gen, generated := newProvenanceStampGenerator(t)
	_, err := gen.generate()
	require.NoError(t, err)
	unsigned := generated.target.DeepCopy()
	unsigned.SetName("unsigned")
	unsigned.SetUID("unsigned")
	unsigned.SetAnnotations(nil)
	copied := generated.target.DeepCopy()
	copied.SetName("copied")
	copied.SetUID("copied")
	otherTrigger := generated.target.DeepCopy()
	otherTrigger.SetName("other-trigger")
	otherTrigger.SetUID("other-trigger-target")
	labels := otherTrigger.GetLabels()
	labels[common.GenerateTriggerUIDLabel] = "old-trigger-uid"
	otherTrigger.SetLabels(labels)
	stamp, err := gen.provenance.Sign(t.Context(), gen.policy, otherTrigger)
	require.NoError(t, err)
	otherTrigger.SetAnnotations(map[string]string{provenance.Annotation: stamp})
	otherRule := generated.target.DeepCopy()
	otherRule.SetName("other-rule")
	otherRule.SetUID("other-rule-target")
	labels = otherRule.GetLabels()
	labels[common.GenerateRuleLabel] = "another-rule"
	otherRule.SetLabels(labels)
	stamp, err = gen.provenance.Sign(t.Context(), gen.policy, otherRule)
	require.NoError(t, err)
	otherRule.SetAnnotations(map[string]string{provenance.Annotation: stamp})
	client := &provenanceCleanupClient{Interface: generated, items: []unstructured.Unstructured{*unsigned, *copied, *otherTrigger, *otherRule, *generated.target.DeepCopy()}}
	controller := &GenerateController{client: client, provenance: gen.provenance, log: logr.Discard()}
	rule := kyvernov2.RuleContext{Rule: gen.rule.Name, Trigger: common.ResourceSpecFromUnstructured(gen.trigger)}
	require.NoError(t, controller.handleNonPolicyChanges(gen.policy, rule, &kyvernov2.UpdateRequest{}))
	require.Equal(t, []string{"target"}, client.deleted)
}

func TestCleanupCannotDeleteConcurrentReplacement(t *testing.T) {
	for _, fromStatus := range []bool{false, true} {
		t.Run(fmt.Sprintf("policy-deleted=%t", fromStatus), func(t *testing.T) {
			gen, generated := newProvenanceStampGenerator(t)
			_, err := gen.generate()
			require.NoError(t, err)
			client := &provenanceCleanupClient{Interface: generated, items: []unstructured.Unstructured{*generated.target.DeepCopy()}, beforeDelete: func(obj *unstructured.Unstructured) { obj.SetUID("replacement") }}
			controller := &GenerateController{client: client, provenance: gen.provenance, log: logr.Discard()}
			rule := kyvernov2.RuleContext{Rule: gen.rule.Name, Trigger: common.ResourceSpecFromUnstructured(gen.trigger)}
			ur := &kyvernov2.UpdateRequest{}
			policy := gen.policy
			if fromStatus {
				ur.Status.GeneratedResources = []kyvernov1.ResourceSpec{common.ResourceSpecFromUnstructured(*generated.target)}
				ur.Spec.Policy = gen.policy.GetName()
				ur.SetAnnotations(map[string]string{provenance.CleanupPolicyUIDAnnotation: string(gen.policy.GetUID())})
				policy = nil
			}
			require.Error(t, controller.deleteDownstream(policy, rule, ur))
			require.Empty(t, client.deleted)
		})
	}
}

func TestPolicyDeletionRequiresAuthenticatedRecord(t *testing.T) {
	for _, missing := range []string{"none", "policy-uid", "target-uid", "stamp"} {
		t.Run(missing, func(t *testing.T) {
			gen, generated := newProvenanceStampGenerator(t)
			_, err := gen.generate()
			require.NoError(t, err)
			client := &provenanceCleanupClient{Interface: generated, items: []unstructured.Unstructured{*generated.target.DeepCopy()}}
			controller := &GenerateController{client: client, provenance: gen.provenance, log: logr.Discard()}
			spec := common.ResourceSpecFromUnstructured(*generated.target)
			ur := &kyvernov2.UpdateRequest{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{provenance.CleanupPolicyUIDAnnotation: string(gen.policy.GetUID())}}, Spec: kyvernov2.UpdateRequestSpec{Policy: gen.policy.GetName()}, Status: kyvernov2.UpdateRequestStatus{GeneratedResources: []kyvernov1.ResourceSpec{spec}}}
			switch missing {
			case "policy-uid":
				ur.SetAnnotations(nil)
			case "target-uid":
				ur.Status.GeneratedResources[0].UID = ""
			case "stamp":
				client.items[0].SetAnnotations(nil)
			}
			require.NoError(t, controller.deleteDownstream(nil, kyvernov2.RuleContext{}, ur))
			if missing == "none" {
				require.Equal(t, []string{"target"}, client.deleted)
			} else {
				require.Empty(t, client.deleted)
			}
		})
	}
}
