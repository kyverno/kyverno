package autogen

import (
	"maps"
	"slices"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
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
		Target: policiesv1alpha1.Target{
			Group:    "apps",
			Version:  "v1",
			Resource: "daemonsets",
			Kind:     "DaemonSet",
		},
		ReplacementsRef: AutogenDefaults,
	},
	"deployments": {
		Target: policiesv1alpha1.Target{
			Group:    "apps",
			Version:  "v1",
			Resource: "deployments",
			Kind:     "Deployment",
		},
		ReplacementsRef: AutogenDefaults,
	},
	"replicasets": {
		Target: policiesv1alpha1.Target{
			Group:    "apps",
			Version:  "v1",
			Resource: "replicasets",
			Kind:     "ReplicaSet",
		},
		ReplacementsRef: AutogenDefaults,
	},
	"statefulsets": {
		Target: policiesv1alpha1.Target{
			Group:    "apps",
			Version:  "v1",
			Resource: "statefulsets",
			Kind:     "StatefulSet",
		},
		ReplacementsRef: AutogenDefaults,
	},
	"jobs": {
		Target: policiesv1alpha1.Target{
			Group:    "batch",
			Version:  "v1",
			Resource: "jobs",
			Kind:     "Job",
		},
		ReplacementsRef: AutogenDefaults,
	},
	"cronjobs": {
		Target: policiesv1alpha1.Target{
			Group:    "batch",
			Version:  "v1",
			Resource: "cronjobs",
			Kind:     "CronJob",
		},
		ReplacementsRef: AutogenCronjobs,
	},
}

var AllConfigs = sets.New(slices.Collect(maps.Keys(ConfigsMap))...)
