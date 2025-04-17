package autogen

import (
	"k8s.io/apimachinery/pkg/util/sets"
)

type replacement struct {
	from string
	to   string
}

type replacements struct {
	entries []replacement
}

type target struct {
	group    string
	version  string
	resource string
	kind     string
}

type config struct {
	target       target
	replacements *replacements
}

var replacementsMap = map[string]*replacements{
	"": {
		entries: []replacement{{
			from: "spec",
			to:   "spec.template.spec",
		}, {
			from: "metadata",
			to:   "spec.template.metadata",
		}},
	},
	"cronjobs": {
		entries: []replacement{{
			from: "spec",
			to:   "spec.jobTemplate.spec.template.spec",
		}, {
			from: "metadata",
			to:   "spec.jobTemplate.spec.template.metadata",
		}},
	},
}

var builtins = map[string]*config{
	"daemonsets": {
		target: target{
			group:    "apps",
			version:  "v1",
			resource: "daemonsets",
			kind:     "DaemonSet",
		},
		replacements: replacementsMap[""],
	},
	"deployments": {
		target: target{
			group:    "apps",
			version:  "v1",
			resource: "deployments",
			kind:     "Deployment",
		},
		replacements: replacementsMap[""],
	},
	"replicasets": {
		target: target{
			group:    "apps",
			version:  "v1",
			resource: "replicasets",
			kind:     "ReplicaSet",
		},
		replacements: replacementsMap[""],
	},
	"statefulsets": {
		target: target{
			group:    "apps",
			version:  "v1",
			resource: "statefulsets",
			kind:     "StatefulSet",
		},
		replacements: replacementsMap[""],
	},
	"jobs": {
		target: target{
			group:    "batch",
			version:  "v1",
			resource: "jobs",
			kind:     "Job",
		},
		replacements: replacementsMap[""],
	},
	"cronjobs": {
		target: target{
			group:    "batch",
			version:  "v1",
			resource: "cronjobs",
			kind:     "CronJob",
		},
		replacements: replacementsMap["cronjobs"],
	},
}

var allConfigs = sets.New("daemonsets", "deployments", "replicasets", "statefulsets", "cronjobs", "jobs")
