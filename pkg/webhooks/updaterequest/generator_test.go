package updaterequest

import (
	"context"
	"testing"
	"time"

	"github.com/alitto/pond/v2"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	fakekyvernov2 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	kyvernoinformers "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	generatorutils "github.com/kyverno/kyverno/pkg/utils/generator"
	"github.com/stretchr/testify/assert"
)

func Test_NewGenerator(t *testing.T) {
	client := fakekyvernov2.NewSimpleClientset()
	informers := kyvernoinformers.NewSharedInformerFactory(client, 0)
	urInformer := informers.Kyverno().V2().UpdateRequests()
	pool := pond.NewPool(2, pond.WithQueueSize(10))

	gen := NewGenerator(client, urInformer, nil, pool)
	assert.NotNil(t, gen)
}

func Test_Generator_Apply(t *testing.T) {
	client := fakekyvernov2.NewSimpleClientset()
	informers := kyvernoinformers.NewSharedInformerFactory(client, 0)
	urInformer := informers.Kyverno().V2().UpdateRequests()
	pool := pond.NewPool(2, pond.WithQueueSize(10))

	urGenerator := generatorutils.NewUpdateRequestGenerator(nil, nil)
	gen := NewGenerator(client, urInformer, urGenerator, pool)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Empty Generate UR should return nil directly without submitting to pool
	emptyUR := kyvernov2.UpdateRequestSpec{
		Type: kyvernov2.Generate,
	}
	err := gen.Apply(ctx, emptyUR)
	assert.NoError(t, err)

	// Valid Mutate UR should be submitted to pool
	mutateUR := kyvernov2.UpdateRequestSpec{
		Type: kyvernov2.Mutate,
	}
	err = gen.Apply(ctx, mutateUR)
	assert.NoError(t, err)
}
