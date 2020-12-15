package utils

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	client "github.com/kyverno/kyverno/pkg/dclient"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/minio/minio/pkg/wildcard"
	"k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var regexVersion = regexp.MustCompile(`v(\d+).(\d+).(\d+)\.*`)

//Contains Check if strint is contained in a list of string
func contains(list []string, element string, fn func(string, string) bool) bool {
	for _, e := range list {
		if fn(e, element) {
			return true
		}
	}
	return false
}

//ContainsNamepace check if namespace satisfies any list of pattern(regex)
func ContainsNamepace(patterns []string, ns string) bool {
	return contains(patterns, ns, compareNamespaces)
}

//ContainsString check if the string is contains in a list
func ContainsString(list []string, element string) bool {
	return contains(list, element, compareString)
}

func compareNamespaces(pattern, ns string) bool {
	return wildcard.Match(pattern, ns)
}

func compareString(str, name string) bool {
	return str == name
}

//NewKubeClient returns a new kubernetes client
func NewKubeClient(config *rest.Config) (kubernetes.Interface, error) {
	kclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return kclient, nil
}

//CRDInstalled to check if the CRD is installed or not
func CRDInstalled(discovery client.IDiscovery, log logr.Logger) bool {
	logger := log.WithName("CRDInstalled")
	check := func(kind string) bool {
		gvr, err := discovery.GetGVRFromKind(kind)
		if err != nil {
			if isServerUnavailable(err) {
				logger.Info("**WARNING** unable to check CRD status", "kind", kind, "error", err.Error())
				return true
			}

			logger.Error(err, "failed to check CRD status", "kind", kind)
			return false
		}

		if reflect.DeepEqual(gvr, schema.GroupVersionResource{}) {
			logger.Info("CRD not installed", "kind", kind)
			return false
		}
		logger.Info("CRD found", "kind", kind)
		return true
	}

	kyvernoCRDs := []string{"ClusterPolicy", "ClusterPolicyReport", "PolicyReport", "ClusterReportChangeRequest", "ReportChangeRequest"}
	for _, crd := range kyvernoCRDs {
		if !check(crd) {
			return false
		}
	}
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
		logger.Error(err, "serverVersion", serverVersion)
		return false
	}

	return b
}

func isVersionHigher(version string, major int, minor int, patch int) (bool, error) {
	groups := regexVersion.FindAllStringSubmatch(version, -1)
	if len(groups) != 1 || len(groups[0]) != 4 {
		return false, fmt.Errorf("invalid version %s. Expected {major}.{minor}.{patch}", version)
	}

	currentMajor, err := strconv.Atoi(groups[0][1])
	if err != nil {
		return false, fmt.Errorf("failed to extract major version from %s", version)
	}

	currentMinor, err := strconv.Atoi(groups[0][2])
	if err != nil {
		return false, fmt.Errorf("failed to extract minor version from %s", version)
	}

	currentPatch, err := strconv.Atoi(groups[0][3])
	if err != nil {
		return false, fmt.Errorf("failed to extract minor version from %s", version)
	}

	if currentMajor <= major && currentMinor <= minor && currentPatch <= patch {
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

func isServerUnavailable(err error) bool {
	// error message -
	// https://github.com/kubernetes/apimachinery/blob/2456ebdaba229616fab2161a615148884b46644b/pkg/api/errors/errors.go#L432
	return strings.Contains(err.Error(), "the server is currently unable to handle the request")
}
