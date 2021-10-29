package utils

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	common "github.com/kyverno/kyverno/pkg/common"
	client "github.com/kyverno/kyverno/pkg/dclient"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/minio/pkg/wildcard"
	"k8s.io/api/admission/v1beta1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var regexVersion = regexp.MustCompile(`v(\d+).(\d+).(\d+)\.*`)

// Contains checks if a string is contained in a list of string
func contains(list []string, element string, fn func(string, string) bool) bool {
	for _, e := range list {
		if fn(e, element) {
			return true
		}
	}
	return false
}

func ContainsPod(list []string, element string) bool {
	for _, e := range list {
		_, k := common.GetKindFromGVK(e)
		if k == element {
			return true
		}
	}
	return false
}

// ContainsNamepace check if namespace satisfies any list of pattern(regex)
func ContainsNamepace(patterns []string, ns string) bool {
	return contains(patterns, ns, compareNamespaces)
}

// ContainsString checks if the string is contained in the list
func ContainsString(list []string, element string) bool {
	return contains(list, element, compareString)
}

func compareNamespaces(pattern, ns string) bool {
	return wildcard.Match(pattern, ns)
}

func compareString(str, name string) bool {
	return str == name
}

// NewKubeClient returns a new kubernetes client
func NewKubeClient(config *rest.Config) (kubernetes.Interface, error) {
	kclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return kclient, nil
}

// CRDsInstalled checks if the Kyverno CRDs are installed or not
func CRDsInstalled(discovery client.IDiscovery) bool {
	kyvernoCRDs := []string{"ClusterPolicy", "ClusterPolicyReport", "PolicyReport", "ClusterReportChangeRequest", "ReportChangeRequest"}
	for _, crd := range kyvernoCRDs {
		if !isCRDInstalled(discovery, crd) {
			return false
		}
	}

	return true
}

func isCRDInstalled(discoveryClient client.IDiscovery, kind string) bool {
	gvr, err := discoveryClient.GetGVRFromKind(kind)
	if gvr.Empty() {
		if err == nil {
			err = fmt.Errorf("not found")
		}

		log.Log.Error(err, "failed to retrieve CRD", "kind", kind)
		return false
	}

	log.Log.Info("CRD found", "gvr", gvr.String())
	return true
}

// ExtractResources extracts the new and old resource as unstructured
func ExtractResources(newRaw []byte, request *v1beta1.AdmissionRequest) (unstructured.Unstructured, unstructured.Unstructured, error) {
	var emptyResource unstructured.Unstructured
	var newResource unstructured.Unstructured
	var oldResource unstructured.Unstructured
	var err error

	// New Resource
	if newRaw == nil {
		newRaw = request.Object.Raw
	}

	if newRaw != nil {
		newResource, err = ConvertResource(newRaw, request.Kind.Group, request.Kind.Version, request.Kind.Kind, request.Namespace)
		if err != nil {
			return emptyResource, emptyResource, fmt.Errorf("failed to convert new raw to unstructured: %v", err)
		}
	}

	// Old Resource
	oldRaw := request.OldObject.Raw
	if oldRaw != nil {
		oldResource, err = ConvertResource(oldRaw, request.Kind.Group, request.Kind.Version, request.Kind.Kind, request.Namespace)
		if err != nil {
			return emptyResource, emptyResource, fmt.Errorf("failed to convert old raw to unstructured: %v", err)
		}
	}

	return newResource, oldResource, err
}

// ConvertResource converts raw bytes to an unstructured object
func ConvertResource(raw []byte, group, version, kind, namespace string) (unstructured.Unstructured, error) {
	obj, err := engineutils.ConvertToUnstructured(raw)
	if err != nil {
		return unstructured.Unstructured{}, fmt.Errorf("failed to convert raw to unstructured: %v", err)
	}

	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: group, Version: version, Kind: kind})

	if namespace != "" && kind != "Namespace" {
		obj.SetNamespace(namespace)
	}

	if obj.GetKind() == "Namespace" && obj.GetNamespace() != "" {
		obj.SetNamespace("")
	}

	return *obj, nil
}

// HigherThanKubernetesVersion compare Kubernetes client version to user given version
func HigherThanKubernetesVersion(client *client.Client, log logr.Logger, major, minor, patch int) bool {
	logger := log.WithName("CompareKubernetesVersion")
	serverVersion, err := client.DiscoveryClient.GetServerVersion()
	if err != nil {
		logger.Error(err, "Failed to get kubernetes server version")
		return false
	}

	b, err := isVersionHigher(serverVersion.String(), major, minor, patch)
	if err != nil {
		logger.Error(err, "serverVersion", serverVersion.String())
		return false
	}

	return b
}

func isVersionHigher(version string, major int, minor int, patch int) (bool, error) {
	groups := regexVersion.FindStringSubmatch(version)
	if len(groups) != 4 {
		return false, fmt.Errorf("invalid version %s. Expected {major}.{minor}.{patch}", version)
	}

	currentMajor, err := strconv.Atoi(groups[1])
	if err != nil {
		return false, fmt.Errorf("failed to extract major version from %s", version)
	}

	currentMinor, err := strconv.Atoi(groups[2])
	if err != nil {
		return false, fmt.Errorf("failed to extract minor version from %s", version)
	}

	currentPatch, err := strconv.Atoi(groups[3])
	if err != nil {
		return false, fmt.Errorf("failed to extract minor version from %s", version)
	}

	if currentMajor < major ||
		(currentMajor == major && currentMinor < minor) ||
		(currentMajor == major && currentMinor == minor && currentPatch <= patch) {
		return false, nil
	}

	return true, nil
}

// SliceContains checks whether values are contained in slice
func SliceContains(slice []string, values ...string) bool {

	var sliceElementsMap = make(map[string]bool, len(slice))
	for _, sliceElement := range slice {
		sliceElementsMap[sliceElement] = true
	}

	for _, value := range values {
		if sliceElementsMap[value] {
			return true
		}
	}

	return false
}

// ApiextensionsJsonToKyvernoConditions takes in user-provided conditions in abstract apiextensions.JSON form
// and converts it into []kyverno.Condition or kyverno.AnyAllConditions according to its content.
// it also helps in validating the condtions as it returns an error when the conditions are provided wrongfully by the user.
func ApiextensionsJsonToKyvernoConditions(original apiextensions.JSON) (interface{}, error) {
	path := "preconditions/validate.deny.conditions"

	// checks for the existence any other field apart from 'any'/'all' under preconditions/validate.deny.conditions
	unknownFieldChecker := func(jsonByteArr []byte, path string) error {
		allowedKeys := map[string]bool{
			"any": true,
			"all": true,
		}
		var jsonDecoded map[string]interface{}
		if err := json.Unmarshal(jsonByteArr, &jsonDecoded); err != nil {
			return fmt.Errorf("error occurred while checking for unknown fields under %s: %+v", path, err)
		}
		for k := range jsonDecoded {
			if !allowedKeys[k] {
				return fmt.Errorf("unknown field '%s' found under %s", k, path)
			}
		}
		return nil
	}

	// marshalling the abstract apiextensions.JSON back to JSON form
	jsonByte, err := json.Marshal(original)
	if err != nil {
		return nil, fmt.Errorf("error occurred while marshalling %s: %+v", path, err)
	}

	var kyvernoOldConditions []kyverno.Condition
	if err = json.Unmarshal(jsonByte, &kyvernoOldConditions); err == nil {
		return kyvernoOldConditions, nil
	}

	var kyvernoAnyAllConditions kyverno.AnyAllConditions
	if err = json.Unmarshal(jsonByte, &kyvernoAnyAllConditions); err == nil {
		// checking if unknown fields exist or not
		err = unknownFieldChecker(jsonByte, path)
		if err != nil {
			return nil, fmt.Errorf("error occurred while parsing %s: %+v", path, err)
		}
		return kyvernoAnyAllConditions, nil
	}
	return nil, fmt.Errorf("error occurred while parsing %s: %+v", path, err)
}
