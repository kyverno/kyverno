package autogen

import (
	"maps"
	"slices"

	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	AutogenDefaults = ""
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
		Target: Target{
			Group:    "apps",
			Version:  "v1",
			Resource: "daemonsets",
			Kind:     "DaemonSet",
		},
		ReplacementsRef: AutogenDefaults,
	},
	"deployments": {
		Target: Target{
			Group:    "apps",
			Version:  "v1",
			Resource: "deployments",
			Kind:     "Deployment",
		},
		ReplacementsRef: AutogenDefaults,
	},
	"replicasets": {
		Target: Target{
			Group:    "apps",
			Version:  "v1",
			Resource: "replicasets",
			Kind:     "ReplicaSet",
		},
		ReplacementsRef: AutogenDefaults,
	},
	"statefulsets": {
		Target: Target{
			Group:    "apps",
			Version:  "v1",
			Resource: "statefulsets",
			Kind:     "StatefulSet",
		},
		ReplacementsRef: AutogenDefaults,
	},
	"jobs": {
		Target: Target{
			Group:    "batch",
			Version:  "v1",
			Resource: "jobs",
			Kind:     "Job",
		},
		ReplacementsRef: AutogenDefaults,
	},
	"cronjobs": {
		Target: Target{
			Group:    "batch",
			Version:  "v1",
			Resource: "cronjobs",
			Kind:     "CronJob",
		},
		ReplacementsRef: AutogenCronjobs,
	},
}

var AllConfigs = sets.New(slices.Collect(maps.Keys(ConfigsMap))...)
