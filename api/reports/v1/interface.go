/*
Copyright 2020 The Kubernetes authors.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	openreportsv1alpha1 "openreports.io/apis/openreports.io/v1alpha1"
)

// +kubebuilder:object:generate=false

// ReportInterface abstracts the concrete report change request type
type ReportInterface interface {
	metav1.Object
	GetResults() []openreportsv1alpha1.ReportResult
	SetResults([]openreportsv1alpha1.ReportResult)
	SetSummary(openreportsv1alpha1.ReportSummary)
}
