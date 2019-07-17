package client

import (
	"errors"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/info"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
)

const (
	// Kind names are case sensitive
	//CSRs CertificateSigningRequest
	CSRs string = "CertificateSigningRequest"
	// Secrets Secret
	Secrets string = "Secret"
	// ConfigMaps ConfigMap
	ConfigMaps string = "ConfigMap"
	// Namespaces Namespace
	Namespaces string = "Namespace"
)
const namespaceCreationMaxWaitTime time.Duration = 30 * time.Second
const namespaceCreationWaitInterval time.Duration = 100 * time.Millisecond

//---testing utilities
func NewMockClient(scheme *runtime.Scheme, objects ...runtime.Object) (*Client, error) {
	client := fake.NewSimpleDynamicClient(scheme, objects...)
	// the typed and dynamic client are initalized with similar resources
	kclient := kubernetesfake.NewSimpleClientset(objects...)
	return &Client{
		client:  client,
		kclient: kclient,
	}, nil

}

// NewFakeDiscoveryClient returns a fakediscovery client
func NewFakeDiscoveryClient(registeredResouces []schema.GroupVersionResource) *fakeDiscoveryClient {
	// Load some-preregistd resources
	res := []schema.GroupVersionResource{
		schema.GroupVersionResource{Version: "v1", Resource: "configmaps"},
		schema.GroupVersionResource{Version: "v1", Resource: "endpoints"},
		schema.GroupVersionResource{Version: "v1", Resource: "namespaces"},
		schema.GroupVersionResource{Version: "v1", Resource: "resourcequotas"},
		schema.GroupVersionResource{Version: "v1", Resource: "secrets"},
		schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"},
		schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"},
		schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
		schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"},
	}
	registeredResouces = append(registeredResouces, res...)
	return &fakeDiscoveryClient{registeredResouces: registeredResouces}
}

type fakeDiscoveryClient struct {
	registeredResouces []schema.GroupVersionResource
}

func (c *fakeDiscoveryClient) getGVR(resource string) schema.GroupVersionResource {
	for _, gvr := range c.registeredResouces {
		if gvr.Resource == resource {
			return gvr
		}
	}
	return schema.GroupVersionResource{}
}

func (c *fakeDiscoveryClient) GetGVRFromKind(kind string) schema.GroupVersionResource {
	resource := strings.ToLower(kind) + "s"
	return c.getGVR(resource)
}

func newUnstructured(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
			},
		},
	}
}

func newUnstructuredWithSpec(apiVersion, kind, namespace, name string, spec map[string]interface{}) *unstructured.Unstructured {
	u := newUnstructured(apiVersion, kind, namespace, name)
	u.Object["spec"] = spec
	return u
}

func retry(attempts int, sleep time.Duration, fn func() error) error {
	if err := fn(); err != nil {
		if s, ok := err.(stop); ok {
			return s.error
		}
		if attempts--; attempts > 0 {
			time.Sleep(sleep)
			return retry(attempts, 2*sleep, fn)
		}
		return err
	}
	return nil
}

// Custom error
type stop struct {
	error
}

func GetAnnotations(obj *unstructured.Unstructured) map[string]interface{} {
	var annotationsMaps map[string]interface{}
	unstr := obj.UnstructuredContent()
	metadata, ok := unstr["metadata"]
	if ok {
		metadataMap, ok := metadata.(map[string]interface{})
		if !ok {
			glog.Info("type mismatch")
			return nil
		}
		annotations, ok := metadataMap["annotations"]
		if !ok {
			glog.Info("annotations not present")
			return nil
		}
		annotationsMaps, ok = annotations.(map[string]interface{})
		if !ok {
			glog.Info("type mismatch")
			return nil
		}
	}
	return annotationsMaps
}

func SetAnnotations(obj *unstructured.Unstructured, annotations map[string]interface{}) error {
	unstr := obj.UnstructuredContent()
	metadata, ok := unstr["metadata"]
	if ok {
		metadataMap, ok := metadata.(map[string]interface{})
		if !ok {
			return errors.New("type mismatch")
		}
		metadataMap["annotations"] = annotations
		unstr["metadata"] = metadataMap
		obj.SetUnstructuredContent(unstr)
	}
	return nil
}

type AnnotationPolicies struct {
	// map[policy_name]
	Policies map[string]AnnotationPolicy `json:"policies"`
}

type AnnotationPolicy struct {
	Status string           `json:"status"`
	Rules  []AnnotationRule `json:"rules,omitempty"`
}

type AnnotationRule struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Type    string `json:"type"`
	Changes string `json:"changes"`
}

func getStatus(status bool) string {
	if status {
		return "Success"
	}
	return "Failure"
}

func getRules(rules []*info.RuleInfo) []AnnotationRule {
	var annrules []AnnotationRule
	for _, r := range rules {
		annrule := AnnotationRule{Name: r.Name,
			Status: getStatus(r.IsSuccessful())}
		//TODO: add mutation changes in policyInfo and in annotation
		annrules = append(annrules, annrule)
	}
	return annrules
}

// input rules can be mutation or validation
func (ap AnnotationPolicy) updateRules(rules interface{}, validation bool) (error, interface{}) {
	ruleList, ok := rules.([]interface{})
	updated := false
	if !ok {
		return errors.New("type mismatch"), false
	}

	// for mutation rule check if the rules are same
	// var mode string
	// if validation {
	//  mode = "Validation"
	// } else {
	//  mode = "Mutation"
	// }
	// // if lengths are differrent then update
	// if len(ruleList) != len(ap.Rules) {
	//  return nil, ap.updateRules
	// }
	// check if there is any update in the rules
	// order of rules is assumed same while comparison
	for i, r := range ruleList {
		rule, ok := r.(map[string]interface{})
		if !ok {
			return errors.New("type mismatch"), nil
		}
		// Name
		name, ok := rule["name"].(string)
		if !ok {
			return errors.New("type mismatch"), nil
		}
		if name != ap.Rules[i].Name {
			updated = true
			break
		}
		// Status
		status, ok := rule["status"].(string)
		if !ok {
			return errors.New("type mismatch"), nil
		}
		if status != ap.Rules[i].Status {
			updated = true
			break
		}
	}
	if updated {
		return nil, ap.Rules
	}
	return nil, nil
}

func newAnnotationPolicy(pi *info.PolicyInfo) AnnotationPolicy {
	status := getStatus(pi.IsSuccessful())
	rules := getRules(pi.Rules)
	return AnnotationPolicy{Status: status,
		Rules: rules}
}

//func GetPolicies(policies interface{}) map[string]
func AddPolicy(pi *info.PolicyInfo, ann map[string]interface{}, validation bool) (error, map[string]interface{}) {
	// Lets build the policy annotation struct from policyInfo
	annpolicy := newAnnotationPolicy(pi)
	// Add policy to annotations
	// If policy does not exist -> Add
	// If already exists then update the status and rules
	policies, ok := ann["policies"]
	if ok {
		policiesMap, ok := policies.(map[string]interface{})
		if !ok {
			glog.Info("type mismatch")
			return errors.New("type mismatch"), nil
		}
		// check if policy record is present
		policy, ok := policiesMap[pi.Name]
		if !ok {
			// not present then we add
			policiesMap[pi.Name] = annpolicy
			ann["policies"] = policiesMap
			return nil, ann
		}
		policyMap, ok := policy.(map[string]interface{})
		if !ok {
			return errors.New("type mismatch"), nil
		}
		// We just update the annotations
		// status
		status := policyMap["status"]
		statusStr, ok := status.(string)
		if !ok {
			return errors.New("type mismatch"), nil
		}
		if statusStr != annpolicy.Status {
			policyMap["status"] = annpolicy.Status
		}
		// check rules
		rules, ok := policyMap["rules"]
		if !ok {
			return errors.New("no rules"), nil
		}
		err, newRules := annpolicy.updateRules(rules, validation)
		if err != nil {
			return err, nil
		}
		if newRules == nil {
			//nothing to update
			return nil, nil
		}
		// update the new rule
		policyMap["rules"] = newRules
		// update policies map
		policiesMap[pi.Name] = policyMap
		ann["policies"] = policiesMap
		return nil, ann
	}
	return nil, nil
}

// RemovePolicy
func RemovePolicy(pi *info.PolicyInfo, ann map[string]interface{}) (error, map[string]interface{}) {
	policies, ok := ann["policies"]
	if ok {
		policiesMap, ok := policies.(map[string]interface{})
		if !ok {
			glog.Info("type mismatch")
			return errors.New("type mismatch"), nil
		}
		// check if policy record is present
		_, ok = policiesMap[pi.Name]
		if ok {
			// delete the pair
			delete(policiesMap, pi.Name)
			ann["policies"] = policiesMap
			return nil, ann
		}
	}
	return nil, nil
}
