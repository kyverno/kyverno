package autogenv2

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	// PodControllerCronJob represent CronJob string
	PodControllerCronJob = "CronJob"
)

var (
	PodControllers         = sets.New("DaemonSet", "Deployment", "Job", "StatefulSet", "ReplicaSet", "ReplicationController", "CronJob")
	podControllersKindsSet = PodControllers.Union(sets.New("Pod"))
)

// AutogenV2 defines the interface for the new autogeneration strategy.
type AutogenV2 interface {
	ExtractPodSpec(resource unstructured.Unstructured) (*unstructured.Unstructured, error)
}

// ImplAutogenV2 is the implementation of the AutogenV2 interface.
type ImplAutogenV2 struct{}

// NewAutogenV2 creates a new instance of AutogenV2.
func NewAutogenV2() AutogenV2 {
	return &ImplAutogenV2{}
}

func splitKinds(controllers, separator string) []string {
	kinds := strings.Split(controllers, separator)
	sort.Strings(kinds)
	return kinds
}

func isKindOtherthanPod(kinds []string) bool {
	if len(kinds) > 1 && kubeutils.ContainsKind(kinds, "Pod") {
		return true
	}
	return false
}

func checkAutogenSupport(needed *bool, subjects ...kyvernov1.ResourceDescription) bool {
	for _, subject := range subjects {
		if subject.Name != "" || len(subject.Names) > 0 || subject.Selector != nil || subject.Annotations != nil || isKindOtherthanPod(subject.Kinds) {
			return false
		}
		if needed != nil {
			*needed = *needed || podControllersKindsSet.HasAny(subject.Kinds...)
		}
	}
	return true
}

// CanAutoGen checks whether the rule(s) (in policy) can be applied to Pod controllers
// returns controllers as:
// - "" if:
//   - name or selector is defined
//   - mixed kinds (Pod + pod controller) is defined
//   - Pod and PodControllers are not defined
//   - mutate.Patches/mutate.PatchesJSON6902/validate.deny/generate rule is defined
//
// - otherwise it returns all pod controllers
func CanAutoGen(spec *kyvernov1.Spec) (applyAutoGen bool, controllers sets.Set[string]) {
	needed := false
	for _, rule := range spec.Rules {
		if rule.HasGenerate() {
			return false, sets.New("none")
		}
		if rule.Mutation != nil {
			if rule.Mutation.PatchesJSON6902 != "" {
				return false, sets.New("none")
			}
			for _, foreach := range rule.Mutation.ForEachMutation {
				if foreach.PatchesJSON6902 != "" {
					return false, sets.New("none")
				}
			}
		}
		match := rule.MatchResources
		if !checkAutogenSupport(&needed, match.ResourceDescription) {
			debug.Info("skip generating rule on pod controllers: Name / Selector in resource description may not be applicable.", "rule", rule.Name)
			return false, sets.New[string]()
		}
		for _, value := range match.Any {
			if !checkAutogenSupport(&needed, value.ResourceDescription) {
				debug.Info("skip generating rule on pod controllers: Name / Selector in match any block is not applicable.", "rule", rule.Name)
				return false, sets.New[string]()
			}
		}
		for _, value := range match.All {
			if !checkAutogenSupport(&needed, value.ResourceDescription) {
				debug.Info("skip generating rule on pod controllers: Name / Selector in match all block is not applicable.", "rule", rule.Name)
				return false, sets.New[string]()
			}
		}
		if exclude := rule.ExcludeResources; exclude != nil {
			if !checkAutogenSupport(&needed, exclude.ResourceDescription) {
				debug.Info("skip generating rule on pod controllers: Name / Selector in resource description may not be applicable.", "rule", rule.Name)
				return false, sets.New[string]()
			}
			for _, value := range exclude.Any {
				if !checkAutogenSupport(&needed, value.ResourceDescription) {
					debug.Info("skip generating rule on pod controllers: Name / Selector in exclude any block is not applicable.", "rule", rule.Name)
					return false, sets.New[string]()
				}
			}
			for _, value := range exclude.All {
				if !checkAutogenSupport(&needed, value.ResourceDescription) {
					debug.Info("skip generating rule on pod controllers: Name / Selector in exclud all block is not applicable.", "rule", rule.Name)
					return false, sets.New[string]()
				}
			}
		}
	}
	if !needed {
		return false, sets.New[string]()
	}
	return true, PodControllers
}

func GetSupportedControllers(spec *kyvernov1.Spec) sets.Set[string] {
	apply, controllers := CanAutoGen(spec)
	if !apply || (controllers.Len() == 1 && controllers.Has("none")) {
		return nil
	}
	return controllers
}

// GetRequestedControllers returns the requested autogen controllers based on object annotations.
func GetRequestedControllers(meta *metav1.ObjectMeta) sets.Set[string] {
	annotations := meta.GetAnnotations()
	if annotations == nil {
		return nil
	}
	controllers, ok := annotations[kyverno.AnnotationAutogenControllers]
	if !ok || controllers == "" {
		return nil
	}
	if controllers == "none" {
		return sets.New[string]()
	}
	return sets.New(splitKinds(controllers, ",")...)
}

// GetControllers computes the autogen controllers that should be applied to a policy.
// It returns the requested, supported and effective controllers (intersection of requested and supported ones).
func GetControllers(meta *metav1.ObjectMeta, spec *kyvernov1.Spec) ([]string, []string, []string) {
	// compute supported and requested controllers
	supported, requested := GetSupportedControllers(spec), GetRequestedControllers(meta)
	// no specific request, we can return supported controllers without further filtering
	if requested == nil {
		return requested.UnsortedList(), supported.UnsortedList(), supported.UnsortedList()
	}
	// filter supported controllers, keeping only those that have been requested
	var activated []string
	for _, controller := range supported.UnsortedList() {
		if requested.Has(controller) {
			activated = append(activated, controller)
		}
	}

	return requested.UnsortedList(), supported.UnsortedList(), activated
}

// ExtractPodSpec extracts the PodSpec from an unstructured resource if the controller supports autogen.
func (a *ImplAutogenV2) ExtractPodSpec(resource unstructured.Unstructured) (*unstructured.Unstructured, error) {
	kind := resource.GetKind()
	var podSpec map[string]interface{}
	var found bool
	var err error
	switch kind {
	case "Deployment", "StatefulSet", "DaemonSet", "Job", "ReplicaSet", "ReplicationController":
		podSpec, found, err = unstructured.NestedMap(resource.Object, "spec", "template", "spec")
		if err != nil || !found {
			return nil, fmt.Errorf("error extracting pod spec: %v", err)
		}

	case "CronJob":
		jobTemplate, found, err := unstructured.NestedMap(resource.Object, "spec", "jobTemplate", "spec", "template", "spec")
		if err != nil || !found {
			return nil, fmt.Errorf("error extracting pod spec from CronJob: %v", err)
		}
		podSpec = jobTemplate

	default:
		return nil, nil // No pod spec for this kind of resource
	}

	return &unstructured.Unstructured{Object: podSpec}, nil
}
