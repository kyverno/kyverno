package autogen

import (
	"maps"
	"slices"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	AutogenDefaults = "defaults"
	AutogenCronjobs = "cronjobs"
)

var ReplacementsMap = map[string][]Replacement{
	AutogenDefaults: {{
		From: "spec",
		To:   "spec.template.spec",
	}, {
		From: "metadata",
		To:   "spec.template.metadata",
	}},
	AutogenCronjobs: {{
		From: "spec",
		To:   "spec.jobTemplate.spec.template.spec",
	}, {
		From: "metadata",
		To:   "spec.jobTemplate.spec.template.metadata",
	}},
}

var ConfigsMap = map[string]*Config{
	"daemonsets": {
		Target: policiesv1beta1.Target{
			Group:    "apps",
			Version:  "v1",
			Resource: "daemonsets",
			Kind:     "DaemonSet",
		},
		ReplacementsRef: AutogenDefaults,
	},
	"deployments": {
		Target: policiesv1beta1.Target{
			Group:    "apps",
			Version:  "v1",
			Resource: "deployments",
			Kind:     "Deployment",
		},
		ReplacementsRef: AutogenDefaults,
	},
	"replicasets": {
		Target: policiesv1beta1.Target{
			Group:    "apps",
			Version:  "v1",
			Resource: "replicasets",
			Kind:     "ReplicaSet",
		},
		ReplacementsRef: AutogenDefaults,
	},
	"statefulsets": {
		Target: policiesv1beta1.Target{
			Group:    "apps",
			Version:  "v1",
			Resource: "statefulsets",
			Kind:     "StatefulSet",
		},
		ReplacementsRef: AutogenDefaults,
	},
	"jobs": {
		Target: policiesv1beta1.Target{
			Group:    "batch",
			Version:  "v1",
			Resource: "jobs",
			Kind:     "Job",
		},
		ReplacementsRef: AutogenDefaults,
	},
	"cronjobs": {
		Target: policiesv1beta1.Target{
			Group:    "batch",
			Version:  "v1",
			Resource: "cronjobs",
			Kind:     "CronJob",
		},
		ReplacementsRef: AutogenCronjobs,
	},
}

var AllConfigs = sets.New(slices.Collect(maps.Keys(ConfigsMap))...)

// ControllerNames returns the built-in controller resource names, sorted.
// They are the built-in names ResolveConfig handles, but admission does not
// validate controller names, so any value in
// spec.autogen.podControllers.controllers is admitted. Resolution is what
// differs: ResolveConfig accepts a custom controller as
// "<resource>.<version>.<group>" unconditionally, and additionally accepts a
// bare resource name resolvable in the cluster for an already-stored policy or
// for a caller that does not run admission validation. A Kind name such as
// "Deployment" resolves to no rule.
func ControllerNames() []string {
	return slices.Sorted(maps.Keys(ConfigsMap))
}
