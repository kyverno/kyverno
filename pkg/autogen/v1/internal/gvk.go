/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package internal

// PodControllerGVKMap maps simple kind names to fully-qualified GVKs
var PodControllerGVKMap = map[string]string{
	"DaemonSet":             "apps/v1/DaemonSet",
	"Deployment":            "apps/v1/Deployment",
	"Job":                   "batch/v1/Job",
	"StatefulSet":           "apps/v1/StatefulSet",
	"ReplicaSet":            "apps/v1/ReplicaSet",
	"ReplicationController": "v1/ReplicationController",
	"CronJob":               "batch/v1/CronJob",
	"Pod":                   "v1/Pod",
}

// ConvertKindsToGVK converts simple kind names to fully-qualified GVKs
func ConvertKindsToGVK(kinds []string) []string {
	gvkKinds := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		if gvk, ok := PodControllerGVKMap[kind]; ok {
			gvkKinds = append(gvkKinds, gvk)
		} else {
			gvkKinds = append(gvkKinds, kind)
		}
	}
	return gvkKinds
}
