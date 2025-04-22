package autogen

import (
	"maps"
	"slices"

	"k8s.io/apimachinery/pkg/util/sets"
)

var ReplacementsMap = map[string][]Replacement{
	"": {{
		From: "spec",
		To:   "spec.template.spec",
	}, {
		From: "metadata",
		To:   "spec.template.metadata",
	}},
	"cronjobs": {{
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
		ReplacementsRef: "",
	},
	"deployments": {
		Target: Target{
			Group:    "apps",
			Version:  "v1",
			Resource: "deployments",
			Kind:     "Deployment",
		},
		ReplacementsRef: "",
	},
	"replicasets": {
		Target: Target{
			Group:    "apps",
			Version:  "v1",
			Resource: "replicasets",
			Kind:     "ReplicaSet",
		},
		ReplacementsRef: "",
	},
	"statefulsets": {
		Target: Target{
			Group:    "apps",
			Version:  "v1",
			Resource: "statefulsets",
			Kind:     "StatefulSet",
		},
		ReplacementsRef: "",
	},
	"jobs": {
		Target: Target{
			Group:    "batch",
			Version:  "v1",
			Resource: "jobs",
			Kind:     "Job",
		},
		ReplacementsRef: "",
	},
	"cronjobs": {
		Target: Target{
			Group:    "batch",
			Version:  "v1",
			Resource: "cronjobs",
			Kind:     "CronJob",
		},
		ReplacementsRef: "cronjobs",
	},
}

var AllConfigs = sets.New(slices.Collect(maps.Keys(ConfigsMap))...)
