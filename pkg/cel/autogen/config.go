package autogen

import (
	"maps"
	"slices"

	"k8s.io/apimachinery/pkg/util/sets"
)

type replacement struct {
	from string
	to   string
}

type target struct {
	group    string
	version  string
	resource string
	kind     string
}

type config struct {
	target          target
	replacementsRef string
}

const (
	AutogenDefaults = "autogen-defaults"
	AutogenCronjobs = "autogen-cronjobs"
)

var replacementsMap = map[string][]replacement{
	AutogenDefaults: {{
		from: "spec",
		to:   "spec.template.spec",
	}, {
		from: "metadata",
		to:   "spec.template.metadata",
	}},
	AutogenCronjobs: {{
		from: "spec",
		to:   "spec.jobTemplate.spec.template.spec",
	}, {
		from: "metadata",
		to:   "spec.jobTemplate.spec.template.metadata",
	}},
}

var configsMap = map[string]*config{
	"daemonsets": {
		target: target{
			group:    "apps",
			version:  "v1",
			resource: "daemonsets",
			kind:     "DaemonSet",
		},
		replacementsRef: AutogenDefaults,
	},
	"deployments": {
		target: target{
			group:    "apps",
			version:  "v1",
			resource: "deployments",
			kind:     "Deployment",
		},
		replacementsRef: AutogenDefaults,
	},
	"replicasets": {
		target: target{
			group:    "apps",
			version:  "v1",
			resource: "replicasets",
			kind:     "ReplicaSet",
		},
		replacementsRef: AutogenDefaults,
	},
	"statefulsets": {
		target: target{
			group:    "apps",
			version:  "v1",
			resource: "statefulsets",
			kind:     "StatefulSet",
		},
		replacementsRef: AutogenDefaults,
	},
	"jobs": {
		target: target{
			group:    "batch",
			version:  "v1",
			resource: "jobs",
			kind:     "Job",
		},
		replacementsRef: AutogenDefaults,
	},
	"cronjobs": {
		target: target{
			group:    "batch",
			version:  "v1",
			resource: "cronjobs",
			kind:     "CronJob",
		},
		replacementsRef: AutogenCronjobs,
	},
}

var allConfigs = sets.New(slices.Collect(maps.Keys(configsMap))...)
