package generation

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	fakekyvernov1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	kyvernoinformers "github.com/kyverno/kyverno/pkg/client/informers/externalversions"

	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	unstructuredUtils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/event"
	log "github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/webhooks/updaterequest"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	"gotest.tools/assert"
	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_updateFeildsInSourceAndUpdatedResource(t *testing.T) {
	type TestCase struct {
		obj            map[string]interface{}
		newRes         map[string]interface{}
		expectedObj    map[string]interface{}
		expectedNewRes map[string]interface{}
	}

	testcases := []TestCase{
		{
			obj: map[string]interface{}{
				"apiVersion": "v1",
				"data": map[string]interface{}{
					"ca": "-----BEGIN CERTIFICATE-----\nMIID5zCCAs+gAwIBAgIUCl6BKlpe2QiS5IQby6QOW7vexMwwDQYJKoZIhvcNAQEL\nBQAwgYIxCzAJBgNVBAYTAlVTMQswCQYDVQQIDAJDQTENMAsGA1UEBwwEVG93bjEQ\n-----END CERTIFICATE-----",
				},
				"kind": "ConfigMap",
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"imageregistry": "https://hub.docker.com/",
						"kubectl.kubernetes.io/last-applied-configuration": `{"apiVersion":"v1","data":{"ca":"-----BEGIN CERTIFICATE-----\nMIID5zCCAs+gAwIBAgIUCl6BKlpe2QiS5IQby6QOW7vexMwwDQYJKoZIhvcNAQEL\nBQAwgYIxCzAJBgNVBAYTAlVTMQswCQYDVQQIDAJDQTENMAsGA1UEBwwEVG93bjEQ\n-----END CERTIFICATE-----"},"kind":"ConfigMap","metadata":{"annotations":{"imageregistry":"https://hub.docker.com/"},"name":"corp-ca-cert","namespace":"default"}}`,
					},
					"creationTimestamp": "2021-01-09T12:37:26Z",
					"labels":            map[string]interface{}{"generate.kyverno.io/clone-policy-name": "generate-policy"},
					"managedFields": map[string]interface{}{
						"apiVersion": "v1",
						"fieldsType": "FieldsV1",
					},
				},
			},

			newRes: map[string]interface{}{
				"apiVersion": "v1",
				"data": map[string]interface{}{
					"ca": "-----BEGIN CERTIFICATE-----\nMIID5zCCAs+gAwIBAgIUCl6BKlpe2QiS5IQby6QOW7vexMwwDQYJKoZIhvcNAQEL\nBQAwgYIxCzAJBgNVBAYTAlVTMQswCQYDVQQIDAJDQTENMAsGA1UEBwwEVG93bjEQ\n-----END CERTIFICATE-----",
				},
				"kind": "ConfigMap",
				"metadata": map[string]interface{}{
					"creationTimestamp": "2021-01-09T12:37:26Z",
					"managedFields": map[string]interface{}{
						"apiVersion": "v1",
						"fieldsType": "FieldsV1",
					},
				},
			},

			expectedObj: map[string]interface{}{
				"apiVersion": "v1",
				"data": map[string]interface{}{
					"ca": "-----BEGIN CERTIFICATE-----MIID5zCCAs+gAwIBAgIUCl6BKlpe2QiS5IQby6QOW7vexMwwDQYJKoZIhvcNAQELBQAwgYIxCzAJBgNVBAYTAlVTMQswCQYDVQQIDAJDQTENMAsGA1UEBwwEVG93bjEQ-----END CERTIFICATE-----",
				},
				"kind": "ConfigMap",
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"imageregistry": "https://hub.docker.com/",
					},
					"labels": map[string]interface{}{},
				},
			},

			expectedNewRes: map[string]interface{}{
				"apiVersion": "v1",
				"data": map[string]interface{}{
					"ca": "-----BEGIN CERTIFICATE-----MIID5zCCAs+gAwIBAgIUCl6BKlpe2QiS5IQby6QOW7vexMwwDQYJKoZIhvcNAQELBQAwgYIxCzAJBgNVBAYTAlVTMQswCQYDVQQIDAJDQTENMAsGA1UEBwwEVG93bjEQ-----END CERTIFICATE-----",
				},
				"kind":     "ConfigMap",
				"metadata": map[string]interface{}{},
			},
		},
		{
			obj: map[string]interface{}{
				"apiVersion": "v1",
				"data": map[string]interface{}{
					"tls.crt": "MIIC2DCCAcCgAwIBAgIBATANBgkqh",
					"tls.key": "MIIEpgIBAAKCAQEA7yn3bRHQ5FHMQ",
				},
				"kind": "Secret",
				"type": "kubernetes.io/tls",
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"kubectl.kubernetes.io/last-applied-configuration": `{"apiVersion":"v1","data":{"tls.crt":"MIIC2DCCAcCgAwIBAgIBATANBgkqh","tls.key": "MIIEpgIBAAKCAQEA7yn3bRHQ5FHMQ"},"type": "kubernetes.io/tls","kind":"Secret"}`,
					},
					"creationTimestamp": "2021-01-09T12:37:26Z",
					"labels":            map[string]interface{}{"generate.kyverno.io/clone-policy-name": "generate-policy"},
					"managedFields": map[string]interface{}{
						"apiVersion": "v1",
						"fieldsType": "FieldsV1",
					},
				},
			},

			newRes: map[string]interface{}{
				"apiVersion": "v1",
				"data": map[string]interface{}{
					"tls.crt": "MIIC2DCCAcCgAwIBAgIBATANBgkqh",
					"tls.key": "MIIEpgIBAAKCAQEA7yn3bRHQ5FHMQ",
				},
				"kind": "Secret",
				"type": "kubernetes.io/tls",
				"metadata": map[string]interface{}{
					"annotations":       map[string]interface{}{},
					"creationTimestamp": "2021-01-09T12:37:26Z",
					"labels": map[string]interface{}{
						"policy.kyverno.io/gr-name":     "gr-qmjr9",
						"policy.kyverno.io/policy-name": "generate-policy",
					},
					"managedFields": map[string]interface{}{
						"apiVersion": "v1",
						"fieldsType": "FieldsV1",
					},
				},
			},

			expectedObj: map[string]interface{}{
				"apiVersion": "v1",
				"data": map[string]interface{}{
					"tls.crt": "MIIC2DCCAcCgAwIBAgIBATANBgkqh",
					"tls.key": "MIIEpgIBAAKCAQEA7yn3bRHQ5FHMQ",
				},
				"kind": "Secret",
				"type": "kubernetes.io/tls",
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{},
					"labels":      map[string]interface{}{},
				},
			},

			expectedNewRes: map[string]interface{}{
				"apiVersion": "v1",
				"data": map[string]interface{}{
					"tls.crt": "MIIC2DCCAcCgAwIBAgIBATANBgkqh",
					"tls.key": "MIIEpgIBAAKCAQEA7yn3bRHQ5FHMQ",
				},
				"kind": "Secret",
				"type": "kubernetes.io/tls",
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{},
					"labels": map[string]interface{}{
						"policy.kyverno.io/gr-name":     "gr-qmjr9",
						"policy.kyverno.io/policy-name": "generate-policy",
					},
				},
			},
		},
		{
			obj: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"kubectl.kubernetes.io/last-applied-configuration": `{"apiVersion":"v1","kind":"Pod", ...}`,
					},
					"creationTimestamp": "2021-01-09T12:37:26Z",
					"labels":            map[string]interface{}{"generate.kyverno.io/clone-policy-name": "generate-policy"},
					"managedFields": map[string]interface{}{
						"apiVersion": "v1",
						"fieldsType": "FieldsV1",
					},
				},
				"spec": map[string]interface{}{
					"containers": map[string]interface{}{
						"image":           "redis:5.0.4",
						"imagePullPolicy": "IfNotPresent",
						"name":            "redis",
					},
				},
				"status": map[string]interface{}{
					"conditions": map[string]interface{}{
						"lastProbeTime":      "null",
						"lastTransitionTime": "2021-01-19T13:09:14Z",
						"status":             "True",
						"type":               "Initialized",
					},
					"containerStatuses": map[string]interface{}{
						"containerID": `docker://55ad0787835e874b6762ad650af3d36c1`,
						"image":       "redis:5.0.4",
					},
				},
			},

			newRes: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"annotations":       map[string]interface{}{},
					"creationTimestamp": "2021-01-09T12:37:26Z",
					"labels":            map[string]interface{}{},
					"managedFields": map[string]interface{}{
						"apiVersion": "v1",
						"fieldsType": "FieldsV1",
					},
				},
				"spec": map[string]interface{}{
					"containers": map[string]interface{}{
						"image":           "redis:5.0.4",
						"imagePullPolicy": "IfNotPresent",
						"name":            "redis",
					},
				},
				"status": map[string]interface{}{
					"conditions": map[string]interface{}{
						"lastProbeTime":      "null",
						"lastTransitionTime": "2021-01-19T13:09:14Z",
						"status":             "True",
						"type":               "Initialized",
					},
					"containerStatuses": map[string]interface{}{
						"containerID": `docker://55ad0787835e874b6762ad650af3d36c1`,
						"image":       "redis:5.0.4",
					},
				},
			},

			expectedObj: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{},
					"labels":      map[string]interface{}{},
				},
				"spec": map[string]interface{}{
					"containers": map[string]interface{}{
						"image":           "redis:5.0.4",
						"imagePullPolicy": "IfNotPresent",
						"name":            "redis",
					},
				},
			},

			expectedNewRes: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{},
					"labels":      map[string]interface{}{},
				},
				"spec": map[string]interface{}{
					"containers": map[string]interface{}{
						"image":           "redis:5.0.4",
						"imagePullPolicy": "IfNotPresent",
						"name":            "redis",
					},
				},
				"status": map[string]interface{}{
					"conditions": map[string]interface{}{
						"lastProbeTime":      "null",
						"lastTransitionTime": "2021-01-19T13:09:14Z",
						"status":             "True",
						"type":               "Initialized",
					},
					"containerStatuses": map[string]interface{}{
						"containerID": `docker://55ad0787835e874b6762ad650af3d36c1`,
						"image":       "redis:5.0.4",
					},
				},
			},
		},
	}

	for _, tc := range testcases {
		o, n := stripNonPolicyFields(tc.obj, tc.newRes, logr.Discard())
		assert.Assert(t, reflect.DeepEqual(tc.expectedObj, o))
		assert.Assert(t, reflect.DeepEqual(tc.expectedNewRes, n))
	}

}

type clientObject struct {
	object       runtime.Object
	resource     string
	resourceList string
}

func Test_handleUpdateGenerateTargetResource(t *testing.T) {

	tests := []struct {
		name                  string
		namespacePolicy       bool
		ur                    runtime.Object
		triggerResourceJson   []byte
		generatedResourceJson []byte
		sourceResourceJson    []byte
		targetList            string
		sourceList            string
		triggerResource       string
		sourceResource        string
		policyJson            []byte
		urName                string
		expectedUrState       kyvernov1beta1.UpdateRequestState
	}{
		{
			name:            "valid generated source updated",
			namespacePolicy: true,
			policyJson: []byte(`{
				"apiVersion": "kyverno.io/v2beta1",
				"kind": "Policy",
				"metadata": {
					"name": "pol-sync-clone",
					"namespace": "poltest"
				},
				"spec": {
					"rules": [
						{
							"name": "gen-zk",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"ConfigMap"
											]
										}
									}
								]
							},
							"generate": {
								"apiVersion": "v1",
								"kind": "Secret",
								"name": "myclonedsecret",
								"namespace": "poltest",
								"synchronize": true,
								"clone": {
									"namespace": "poltest",
									"name": "regcred"
								}
							}
						}
					]
				}
			}`),
			ur: &kyvernov1beta1.UpdateRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ur-valid",
					Namespace: config.KyvernoNamespace(),
				},
				Status: kyvernov1beta1.UpdateRequestStatus{
					State: kyvernov1beta1.Completed,
				},
			},
			urName:          "ur-valid",
			targetList:      "ConfigMapList",
			triggerResource: "comfigmaps",
			sourceList:      "SecretList",
			generatedResourceJson: []byte(`
			{
				"apiVersion":"v1",
				"data":{
				   "foo":"YmFy"
				},
				"kind":"Secret",
				"metadata":{
				   "labels":{
					  "app.kubernetes.io/managed-by":"kyverno",
					  "kyverno.io/generated-by-kind":"ConfigMap",
					  "kyverno.io/generated-by-name":"cm-2",
					  "kyverno.io/generated-by-namespace":"poltest",
					  "policy.kyverno.io/gr-name":"ur-valid",
					  "policy.kyverno.io/policy-kind":"Namespace",
					  "policy.kyverno.io/policy-name":"pol-sync-clone",
					  "policy.kyverno.io/synchronize":"enable"
				   },
				   "name":"myclonedsecret",
				   "namespace":"poltest"
				}
			 }
				`),
			sourceResource:  "secrets",
			expectedUrState: kyvernov1beta1.Pending,
			sourceResourceJson: []byte(`
			{
				"apiVersion": "v1",
				"data": {
					"foo": "bar"
				},
				"kind": "Secret",
				"metadata": {
					"name": "regcred",
					"namespace": "poltest"
				}
			}
			`),
			triggerResourceJson: []byte(`{
				"apiVersion": "v1",
				"data": {
					"sj": "js"
				},
				"kind": "ConfigMap",
				"metadata": {
					"name": "cm-2",
					"namespace": "poltest"
				}
			}`),
		},
		{
			name:            "valid generated source updated-cluster policy",
			namespacePolicy: false,
			policyJson: []byte(`{
				"apiVersion":"kyverno.io/v1",
				"kind":"ClusterPolicy",
				"metadata":{
				   "name":"pol-sync-clone"
				},
				"spec":{
				   "rules":[
					  {
						 "name":"gen-zk",
						 "match":{
							"any":[
							   {
								  "resources":{
									 "kinds":[
										"ConfigMap"
									 ]
								  }
							   }
							]
						 },
						 "generate":{
							"apiVersion":"v1",
							"kind":"Secret",
							"name":"myclonedsecret",
							"namespace":"poltest",
							"synchronize":true,
							"clone":{
							   "namespace":"poltest",
							   "name":"regcred"
							}
						 }
					  }
				   ]
				}
			 }`),
			ur: &kyvernov1beta1.UpdateRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ur-valid",
					Namespace: config.KyvernoNamespace(),
				},
				Status: kyvernov1beta1.UpdateRequestStatus{
					State: kyvernov1beta1.Completed,
				},
			},
			urName:          "ur-valid",
			targetList:      "ConfigMapList",
			triggerResource: "comfigmaps",
			sourceList:      "SecretList",
			generatedResourceJson: []byte(`
			{
				"apiVersion":"v1",
				"data":{
				   "foo":"YmFy"
				},
				"kind":"Secret",
				"metadata":{
				   "labels":{
					  "app.kubernetes.io/managed-by":"kyverno",
					  "kyverno.io/generated-by-kind":"ConfigMap",
					  "kyverno.io/generated-by-name":"cm-2",
					  "kyverno.io/generated-by-namespace":"poltest",
					  "policy.kyverno.io/gr-name":"ur-valid",
					  "policy.kyverno.io/policy-kind":"Cluster",
					  "policy.kyverno.io/policy-name":"pol-sync-clone",
					  "policy.kyverno.io/synchronize":"enable"
				   },
				   "name":"myclonedsecret",
				   "namespace":"poltest"
				}
			 }
				`),
			sourceResource:  "secrets",
			expectedUrState: kyvernov1beta1.Pending,
			sourceResourceJson: []byte(`
			{
				"apiVersion": "v1",
				"data": {
					"foo": "bar"
				},
				"kind": "Secret",
				"metadata": {
					"name": "regcred",
					"namespace": "poltest"
				}
			}
			`),
			triggerResourceJson: []byte(`{
				"apiVersion": "v1",
				"data": {
					"sj": "js"
				},
				"kind": "ConfigMap",
				"metadata": {
					"name": "cm-2",
					"namespace": "poltest"
				}
			}`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := log.WithName("Test_handleUpdateGenerateTargetResource")
			ctx, cancel := context.WithCancel(context.Background())
			_ = ctx.Done()
			t.Cleanup(cancel)

			var triggerUnstructured *unstructured.Unstructured
			triggerUnstructured, err := unstructuredUtils.ConvertToUnstructured(tt.triggerResourceJson)
			assert.NilError(t, err)

			var generatedResource corev1.Secret
			err = json.Unmarshal(tt.generatedResourceJson, &generatedResource)
			assert.NilError(t, err)

			var sourceResourceUnstructured *unstructured.Unstructured
			sourceResourceUnstructured, err = unstructuredUtils.ConvertToUnstructured(tt.sourceResourceJson)
			assert.NilError(t, err)

			clientObjects := []clientObject{
				{
					object:       triggerUnstructured,
					resource:     tt.triggerResource,
					resourceList: tt.targetList,
				},
				{
					object:       sourceResourceUnstructured,
					resource:     tt.sourceResource,
					resourceList: tt.sourceList,
				},
			}
			var objects []runtime.Object
			if tt.namespacePolicy {
				var nsPolicy kyvernov1.Policy
				err = json.Unmarshal(tt.policyJson, &nsPolicy)
				assert.NilError(t, err)
				objects = append(objects, &nsPolicy, tt.ur)
			} else {
				var clsPolicy kyvernov1.ClusterPolicy
				err = json.Unmarshal(tt.policyJson, &clsPolicy)
				assert.NilError(t, err)
				objects = append(objects, &clsPolicy, tt.ur)
			}

			gh, fakeUrLister, err := newFakeGenerateHandler(&ctx, logger, objects, clientObjects)
			assert.NilError(t, err)
			request := &v1.AdmissionRequest{
				Operation: v1.Update,
				Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"},
				Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "Secret"},
				OldObject: runtime.RawExtension{
					Raw: []byte(tt.generatedResourceJson),
				},
				Object: runtime.RawExtension{
					Raw: []byte(tt.generatedResourceJson),
				},
			}
			var policy []kyvernov1.PolicyInterface
			gh.HandleUpdatesForGenerateRules(request, policy)

			ur, err := (*fakeUrLister).KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Get(ctx, "ur-valid", metav1.GetOptions{})
			assert.NilError(t, err)
			assert.Equal(t, tt.expectedUrState, ur.Status.State)

		})
	}
}

func newFakeGenerateHandler(ctx *context.Context, logger logr.Logger, objects []runtime.Object, clientObjs []clientObject) (GenerationHandler, *fakekyvernov1.Clientset, error) {

	kyvernoClient := fakekyvernov1.NewSimpleClientset(objects...)
	kyvernoInformers := kyvernoinformers.NewSharedInformerFactory(kyvernoClient, 0)
	kyvernoInformers.Start((*ctx).Done())
	kyvernoInformers.WaitForCacheSync((*ctx).Done())

	client := fake.NewSimpleClientset()
	informers := informers.NewSharedInformerFactory(client, 0)
	informers.Start((*ctx).Done())

	urLister := kyvernoInformers.Kyverno().V1beta1().UpdateRequests().Lister().UpdateRequests(config.KyvernoNamespace())
	kyvernoInformers.Start((*ctx).Done())
	kyvernoInformers.WaitForCacheSync((*ctx).Done())

	nsLister := informers.Core().V1().Namespaces().Lister()
	urGenerator := updaterequest.NewFake()
	urUpdater := webhookutils.NewUpdateRequestUpdater(kyvernoClient, urLister)
	eventGen := event.NewFake()

	gvrToListKind := map[schema.GroupVersionResource]string{}
	scheme := runtime.NewScheme()
	gvrs := make([]schema.GroupVersionResource, len(clientObjs))

	clientResources := make([]runtime.Object, len(clientObjs))
	for index, clientObj := range clientObjs {
		gvrs[index] = clientObj.object.GetObjectKind().GroupVersionKind().GroupVersion().WithResource(clientObj.resource)
		gvrToListKind[gvrs[index]] = clientObj.resourceList
		clientResources[index] = clientObj.object
		scheme.AddKnownTypes(clientObj.object.GetObjectKind().GroupVersionKind().GroupVersion(), clientObj.object)
	}

	dclientVar, err := dclient.NewFakeClient(scheme, gvrToListKind, clientResources...)
	if err != nil {
		return nil, nil, err
	}

	dclientVar.SetDiscovery(dclient.NewFakeDiscoveryClient(gvrs))

	fakeGh := NewGenerationHandler(logger, dclientVar, kyvernoClient, nsLister, urLister, urGenerator, urUpdater, eventGen)

	return fakeGh, kyvernoClient, nil
}
