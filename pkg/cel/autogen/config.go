package autogen

import (
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

var replacementsMap = map[string][]replacement{
	"": {{
		from: "spec",
		to:   "spec.template.spec",
	}, {
		from: "metadata",
		to:   "spec.template.metadata",
	}},
	"cronjobs": {{
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
		replacementsRef: "",
	},
	"deployments": {
		target: target{
			group:    "apps",
			version:  "v1",
			resource: "deployments",
			kind:     "Deployment",
		},
		replacementsRef: "",
	},
	"replicasets": {
		target: target{
			group:    "apps",
			version:  "v1",
			resource: "replicasets",
			kind:     "ReplicaSet",
		},
		replacementsRef: "",
	},
	"statefulsets": {
		target: target{
			group:    "apps",
			version:  "v1",
			resource: "statefulsets",
			kind:     "StatefulSet",
		},
		replacementsRef: "",
	},
	"jobs": {
		target: target{
			group:    "batch",
			version:  "v1",
			resource: "jobs",
			kind:     "Job",
		},
		replacementsRef: "",
	},
	"cronjobs": {
		target: target{
			group:    "batch",
			version:  "v1",
			resource: "cronjobs",
			kind:     "CronJob",
		},
		replacementsRef: "cronjobs",
	},
}

var allConfigs = sets.New("daemonsets", "deployments", "replicasets", "statefulsets", "cronjobs", "jobs")
