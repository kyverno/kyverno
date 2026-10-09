package updaterequest

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/generate/provenance"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernofake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	kyvernoinformers "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8stesting "k8s.io/client-go/testing"
)

type persistPolicyIdentityUR struct{}

func (*persistPolicyIdentityUR) Generate(ctx context.Context, client kyvernoclient.Interface, resource *kyvernov2.UpdateRequest, _ logr.Logger) (*kyvernov2.UpdateRequest, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	resource = resource.DeepCopy()
	resource.Name = "ur-policy-identity"
	return client.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Create(ctx, resource, metav1.CreateOptions{})
}

func TestApplyPreservesPolicyUIDAcrossAsyncCancellation(t *testing.T) {
	t.Parallel()
	for _, uid := range []types.UID{"evaluated-policy-uid", ""} {
		name := "bound policy"
		if uid == "" {
			name = "legacy context remains unbound"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			client := kyvernofake.NewSimpleClientset()
			completed := make(chan *kyvernov2.UpdateRequest, 1)
			client.PrependReactor("update", "updaterequests", func(action k8stesting.Action) (bool, runtime.Object, error) {
				if action.GetSubresource() == "status" {
					completed <- action.(k8stesting.UpdateAction).GetObject().(*kyvernov2.UpdateRequest).DeepCopy()
				}
				return false, nil, nil
			})
			informers := kyvernoinformers.NewSharedInformerFactory(client, 0)
			generator := NewGenerator(client, informers.Kyverno().V2().UpdateRequests(), &persistPolicyIdentityUR{})
			ctx, cancel := context.WithCancel(t.Context())
			ctx = WithPolicyUID(ctx, uid)
			cancel()
			require.NoError(t, generator.Apply(ctx, kyvernov2.UpdateRequestSpec{
				Type:        kyvernov2.Generate,
				Policy:      "generate-policy",
				RuleContext: []kyvernov2.RuleContext{{Rule: "generate"}},
			}))
			select {
			case result := <-completed:
				assert.Equal(t, kyvernov2.Pending, result.Status.State)
				assert.Equal(t, string(uid), result.GetAnnotations()[provenance.PolicyUIDAnnotation])
				if uid == "" {
					assert.NotContains(t, result.GetAnnotations(), provenance.PolicyUIDAnnotation)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("asynchronous request did not preserve identity and survive admission cancellation")
			}
		})
	}
}
