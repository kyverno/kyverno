{{/* vim: set filetype=mustache: */}}

{{- define "kyverno.flowcontrol.apiVersion" -}}
{{- if .Capabilities.APIVersions.Has "flowcontrol.apiserver.k8s.io/v1beta3" -}}
  flowcontrol.apiserver.k8s.io/v1beta3
{{- else if .Capabilities.APIVersions.Has "flowcontrol.apiserver.k8s.io/v1beta2" -}}
  flowcontrol.apiserver.k8s.io/v1beta2
{{- else if .Capabilities.APIVersions.Has "flowcontrol.apiserver.k8s.io/v1beta1" -}}
  flowcontrol.apiserver.k8s.io/v1beta1
{{- else -}}
  flowcontrol.apiserver.k8s.io/v1alpha1
{{- end -}}
{{- end -}}
