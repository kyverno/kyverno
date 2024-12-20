package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	kyvFuzz "github.com/kyverno/kyverno/pkg/utils/fuzz"

	corev1 "k8s.io/api/core/v1"

	fuzz "github.com/AdaLogics/go-fuzz-headers"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/factories"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/policycontext"
	"github.com/kyverno/kyverno/pkg/imageverifycache"
	"github.com/kyverno/kyverno/pkg/registryclient"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
)

var (
	fuzzCfg        = config.NewDefaultConfiguration(false)
	fuzzMetricsCfg = config.NewDefaultMetricsConfiguration()
	fuzzJp         = jmespath.New(fuzzCfg)

	validateContext = context.Background()
	regClient       = registryclient.NewOrDie()
	validateEngine  = NewEngine(
		fuzzCfg,
		config.NewDefaultMetricsConfiguration(),
		fuzzJp,
		nil,
		factories.DefaultRegistryClientFactory(adapters.RegistryClient(regClient), nil),
		imageverifycache.DisabledImageVerifyCache(),
		factories.DefaultContextLoaderFactory(nil),
		nil,
	)
	initter sync.Once
)

func buildFuzzContext(ff *fuzz.ConsumeFuzzer) (*PolicyContext, error) {
	cpSpec, err := kyvFuzz.CreatePolicySpec(ff)
	if err != nil {
		return nil, err
	}
	cpol := &kyverno.ClusterPolicy{}
	cpol.Spec = cpSpec

	if len(autogen.ComputeRules(cpol, "")) == 0 {
		return nil, fmt.Errorf("No rules created")
	}

	resourceUnstructured, err := kyvFuzz.CreateUnstructuredObject(ff, "")
	if err != nil {
		return nil, err
	}

	policyContext, err := policycontext.NewPolicyContext(
		fuzzJp,
		*resourceUnstructured,
		kyverno.Create,
		nil,
		fuzzCfg,
	)
	if err != nil {
		return nil, err
	}

	policyContext = policyContext.
		WithPolicy(cpol).
		WithNewResource(*resourceUnstructured)

	addOldResource, err := ff.GetBool()
	if err != nil {
		return nil, err
	}

	if addOldResource {
		oldResourceUnstructured, err := kyvFuzz.CreateUnstructuredObject(ff, "")
		if err != nil {
			return nil, err
		}

		oldResource, err := json.Marshal(oldResourceUnstructured)
		if err != nil {
			return policyContext, nil
		}

		err = enginecontext.AddOldResource(policyContext.JSONContext(), oldResource)
		if err != nil {
			return nil, err
		}

		policyContext = policyContext.WithOldResource(*oldResourceUnstructured)
	}

	return policyContext, nil
}

/*
VerifyAndPatchImage
*/
func FuzzVerifyImageAndPatchTest(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ff := fuzz.NewConsumer(data)
		pc, err := buildFuzzContext(ff)
		if err != nil {
			return
		}

		verifyImageAndPatchEngine := NewEngine(
			fuzzCfg,
			fuzzMetricsCfg,
			fuzzJp,
			nil,
			factories.DefaultRegistryClientFactory(adapters.RegistryClient(registryclient.NewOrDie()), nil),
			imageverifycache.DisabledImageVerifyCache(),
			factories.DefaultContextLoaderFactory(nil),
			nil,
		)

		_, _ = verifyImageAndPatchEngine.VerifyAndPatchImages(
			context.Background(),
			pc,
		)
	})
}

func FuzzEngineValidateTest(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ff := fuzz.NewConsumer(data)
		cpSpec, err := kyvFuzz.CreatePolicySpec(ff)
		if err != nil {
			return
		}
		policy := &kyverno.ClusterPolicy{}
		policy.Spec = cpSpec

		if len(autogen.ComputeRules(policy, "")) == 0 {
			return
		}

		resourceUnstructured, err := kyvFuzz.CreateUnstructuredObject(ff, "")
		if err != nil {
			return
		}

		pc, err := NewPolicyContext(fuzzJp, *resourceUnstructured, kyverno.Create, nil, fuzzCfg)
		if err != nil {
			return
		}

		validateEngine.Validate(
			validateContext,
			pc.WithPolicy(policy),
		)
	})
}

func getPod(ff *fuzz.ConsumeFuzzer) (*corev1.Pod, error) {
	pod := &corev1.Pod{}
	err := ff.GenerateStruct(pod)
	pod.Kind = "Pod"
	pod.APIVersion = "v1"
	return pod, err
}

func FuzzPodBypass(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		initter.Do(kyvFuzz.InitFuzz)

		ff := fuzz.NewConsumer(data)
		policyToCheck, err := ff.GetInt()
		if err != nil {
			return
		}
		testPolicy := kyvFuzz.Policies[policyToCheck%11]

		pod, err := getPod(ff)
		if err != nil {
			return
		}

		shouldBlock, err := testPolicy.ShouldBlock(pod)
		if err != nil {
			return
		}

		resource, err := json.MarshalIndent(pod, "", "  ")
		if err != nil {
			return
		}

		resourceUnstructured, err := kubeutils.BytesToUnstructured(resource)
		if err != nil {
			return
		}

		pc, err := NewPolicyContext(fuzzJp, *resourceUnstructured, kyverno.Create, nil, fuzzCfg)
		if err != nil {
			return
		}
		er := validateEngine.Validate(
			validateContext,
			pc.WithPolicy(testPolicy.ClusterPolicy),
		)
		blocked := blockRequest([]engineapi.EngineResponse{er})
		if blocked != shouldBlock {
			panic(fmt.Sprintf("\nDid not block a resource that should be blocked:\n%s\n should have been blocked by \n%+v\n\nshouldBlock was %t\nblocked was %t\n", string(resource), testPolicy.ClusterPolicy, shouldBlock, blocked))
		}
	})
}

func blockRequest(engineResponses []engineapi.EngineResponse) bool {
	for _, er := range engineResponses {
		if er.IsFailed() {
			return true
		}
	}
	return false
}

func FuzzMutateTest(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {

		ff := fuzz.NewConsumer(data)
		//ff.GenerateStruct(policy)
		cpSpec, err := kyvFuzz.CreatePolicySpec(ff)
		if err != nil {
			return
		}
		policy := &kyverno.ClusterPolicy{}
		policy.Spec = cpSpec

		if len(autogen.ComputeRules(policy, "")) == 0 {
			return
		}

		resource, err := kyvFuzz.CreateUnstructuredObject(ff, "")
		if err != nil {
			return
		}

		// create policy context
		pc, err := NewPolicyContext(
			fuzzJp,
			*resource,
			kyverno.Create,
			nil,
			fuzzCfg,
		)
		if err != nil {
			t.Skip()
		}
		fuzzInterface := kyvFuzz.FuzzInterface{FF: ff}
		e := NewEngine(
			fuzzCfg,
			config.NewDefaultMetricsConfiguration(),
			fuzzJp,
			adapters.Client(fuzzInterface),
			factories.DefaultRegistryClientFactory(adapters.RegistryClient(nil), nil),
			imageverifycache.DisabledImageVerifyCache(),
			factories.DefaultContextLoaderFactory(nil),
			nil,
		)
		e.Mutate(
			context.Background(),
			pc.WithPolicy(policy),
		)
	})
}
