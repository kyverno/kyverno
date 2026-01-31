package imageverification

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/go-logr/logr/testr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	fakekyvernov1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	kyvernoinformers "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	"github.com/kyverno/kyverno/pkg/engine/context/resolvers"
	"github.com/kyverno/kyverno/pkg/engine/factories"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/exceptions"
	"github.com/kyverno/kyverno/pkg/imageverifycache"
	"github.com/kyverno/kyverno/pkg/registryclient"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	"gotest.tools/assert"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

func init() {
	_ = reportutils.NewReportingConfig([]string{"pass", "fail", "warn", "error", "skip"}, "validate", "mutate", "mutateExisting", "generate", "imageVerify")
}

func newFakeImageVerificationHandler(t *testing.T, ctx context.Context) (ImageVerificationHandler, corev1listers.NamespaceLister) {
	client := fake.NewSimpleClientset()
	informers := kubeinformers.NewSharedInformerFactory(client, 0)
	informers.Start(ctx.Done())

	kyvernoclient := fakekyvernov1.NewSimpleClientset()
	kyvernoInformers := kyvernoinformers.NewSharedInformerFactory(kyvernoclient, 0)
	kyvernoInformers.Start(ctx.Done())

	dclientInstance := dclient.NewEmptyFakeClient()
	configuration := config.NewDefaultConfiguration(false)
	jp := jmespath.New(configuration)
	rclient := registryclient.NewOrDie()
	configMapResolver, _ := resolvers.NewClientBasedResolver(client)
	peLister := kyvernoInformers.Kyverno().V2().PolicyExceptions().Lister()

	eng := engine.NewEngine(
		configuration,
		jp,
		adapters.Client(dclientInstance),
		factories.DefaultRegistryClientFactory(adapters.RegistryClient(rclient), nil),
		imageverifycache.DisabledImageVerifyCache(),
		factories.DefaultContextLoaderFactory(configMapResolver),
		exceptions.New(peLister),
		nil,
	)

	logger := testr.New(t)
	return NewImageVerificationHandler(
		logger,
		kyvernoclient,
		eng,
		event.NewFake(),
		false,
		informers.Core().V1().Namespaces().Lister(),
	), informers.Core().V1().Namespaces().Lister()
}

var policyVerifyImage = `{
	"apiVersion": "kyverno.io/v1",
	"kind": "ClusterPolicy",
	"metadata": {
		"name": "check-image",
		"annotations": {
			"pod-policies.kyverno.io/autogen-controllers": "none"
		}
	},
	"spec": {
		"validationFailureAction": "Enforce",
		"background": false,
		"webhookTimeoutSeconds": 30,
		"failurePolicy": "Fail",
		"rules": [
			{
				"name": "check-signature",
				"match": {
					"resources": {
						"kinds": [
							"Pod"
						]
					}
				},
				"verifyImages": [
					{
						"imageReferences": [
							"*"
						],
						"attestors": [
							{
								"entries": [
									{
										"keys": {
											"publicKeys": "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM\n5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==\n-----END PUBLIC KEY-----"
										}
									}
								]
							}
						]
					}
				]
			}
		]
	}
}`

var policyVerifyImageAudit = `{
	"apiVersion": "kyverno.io/v1",
	"kind": "ClusterPolicy",
	"metadata": {
		"name": "check-image-audit",
		"annotations": {
			"pod-policies.kyverno.io/autogen-controllers": "none"
		}
	},
	"spec": {
		"validationFailureAction": "Audit",
		"background": false,
		"webhookTimeoutSeconds": 30,
		"failurePolicy": "Fail",
		"rules": [
			{
				"name": "check-signature-audit",
				"match": {
					"resources": {
						"kinds": [
							"Pod"
						]
					}
				},
				"verifyImages": [
					{
						"imageReferences": [
							"*"
						],
						"attestors": [
							{
								"entries": [
									{
										"keys": {
											"publicKeys": "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM\n5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==\n-----END PUBLIC KEY-----"
										}
									}
								]
							}
						]
					}
				]
			}
		]
	}
}`

var podWithImage = `{
	"apiVersion": "v1",
	"kind": "Pod",
	"metadata": {
		"name": "test-pod",
		"namespace": "default"
	},
	"spec": {
		"containers": [
			{
				"name": "nginx",
				"image": "nginx:latest"
			}
		]
	}
}`

func TestHandle_NoPolicies(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler, _ := newFakeImageVerificationHandler(t, ctx)

	request := admissionv1.AdmissionRequest{
		Operation: admissionv1.Create,
		Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
		Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		Object: runtime.RawExtension{
			Raw: []byte(podWithImage),
		},
		RequestResource: &metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
	}

	configuration := config.NewDefaultConfiguration(false)
	jp := jmespath.New(configuration)
	pcBuilder := webhookutils.NewPolicyContextBuilder(configuration, jp)
	policyContext, err := pcBuilder.Build(request, nil, nil, schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"})
	assert.NilError(t, err)

	patches, warnings, handleErr := handler.Handle(ctx, request, []kyvernov1.PolicyInterface{}, policyContext)

	assert.NilError(t, handleErr)
	assert.Assert(t, patches == nil)
	assert.Equal(t, len(warnings), 0)
}

func TestHandle_ImageVerification_Enforce(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler, nsLister := newFakeImageVerificationHandler(t, ctx)

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
	}

	var policy kyvernov1.ClusterPolicy
	err := json.Unmarshal([]byte(policyVerifyImage), &policy)
	assert.NilError(t, err)

	request := admissionv1.AdmissionRequest{
		Operation: admissionv1.Create,
		Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
		Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		Namespace: "default",
		Object: runtime.RawExtension{
			Raw: []byte(podWithImage),
		},
		RequestResource: &metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
	}

	configuration := config.NewDefaultConfiguration(false)
	jp := jmespath.New(configuration)
	pcBuilder := webhookutils.NewPolicyContextBuilder(configuration, jp)
	policyContext, err := pcBuilder.Build(request, nil, nil, schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"})
	assert.NilError(t, err)

	if err := policyContext.JSONContext().AddNamespace(ns.GetName()); err == nil {
		if ns, err := nsLister.Get(ns.GetName()); err == nil {
			labels := make(map[string]interface{})
			for k, v := range ns.GetLabels() {
				labels[k] = v
			}
			if err := policyContext.JSONContext().AddResource(labels); err != nil {
				t.Logf("Failed to add namespace labels: %v", err)
			}
		}
	}

	_, _, handleErr := handler.Handle(ctx, request, []kyvernov1.PolicyInterface{&policy}, policyContext)
	t.Logf("Test completed, handleErr: %v", handleErr)
}

func TestHandle_ImageVerification_Audit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler, nsLister := newFakeImageVerificationHandler(t, ctx)

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
	}

	var policy kyvernov1.ClusterPolicy
	err := json.Unmarshal([]byte(policyVerifyImageAudit), &policy)
	assert.NilError(t, err)

	request := admissionv1.AdmissionRequest{
		Operation: admissionv1.Create,
		Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
		Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		Namespace: "default",
		Object: runtime.RawExtension{
			Raw: []byte(podWithImage),
		},
		RequestResource: &metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
	}

	configuration := config.NewDefaultConfiguration(false)
	jp := jmespath.New(configuration)
	pcBuilder := webhookutils.NewPolicyContextBuilder(configuration, jp)
	policyContext, err := pcBuilder.Build(request, nil, nil, schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"})
	assert.NilError(t, err)

	if err := policyContext.JSONContext().AddNamespace(ns.GetName()); err == nil {
		if ns, err := nsLister.Get(ns.GetName()); err == nil {
			labels := make(map[string]interface{})
			for k, v := range ns.GetLabels() {
				labels[k] = v
			}
			if err := policyContext.JSONContext().AddResource(labels); err != nil {
				t.Logf("Failed to add namespace labels: %v", err)
			}
		}
	}

	_, _, handleErr := handler.Handle(ctx, request, []kyvernov1.PolicyInterface{&policy}, policyContext)
	
	t.Logf("Test completed, handleErr: %v", handleErr)
}

