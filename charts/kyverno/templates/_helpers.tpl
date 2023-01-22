{{/* vim: set filetype=mustache: */}}

{{- define "kyverno.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "kyverno.fullname" -}}
{{- if .Values.fullnameOverride -}}
  {{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
  {{- $name := default .Chart.Name .Values.nameOverride -}}
  {{- if contains $name .Release.Name -}}
    {{- .Release.Name | trunc 63 | trimSuffix "-" -}}
  {{- else -}}
    {{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
  {{- end -}}
{{- end -}}
{{- end -}}

{{- define "kyverno.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "kyverno.namespace" -}}
{{ default .Release.Namespace .Values.namespaceOverride }}
{{- end -}}

{{/* Helm labels */}}
{{- define "kyverno.helmLabels" -}}
{{- if not .Values.templating.enabled -}}
helm.sh/chart: {{ template "kyverno.chart" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}
{{- end -}}

{{/* Version labels */}}
{{- define "kyverno.versionLabels" -}}
{{- if .Values.templating.enabled -}}
app.kubernetes.io/version: {{ required "templating.version is required when templating.enabled is true" .Values.templating.version | replace "+" "_" }}
{{- else -}}
app.kubernetes.io/version: {{ .Chart.Version | replace "+" "_" }}
{{- end -}}
{{- end -}}

{{/* Helm required labels */}}
{{- define "kyverno.labels" -}}
app.kubernetes.io/component: kyverno
{{- with (include "kyverno.helmLabels" .) }}
{{ . }}
{{- end }}
{{- with (include "kyverno.matchLabels" .) }}
{{ . }}
{{- end }}
app.kubernetes.io/part-of: {{ template "kyverno.name" . }}
{{- with (include "kyverno.versionLabels" .) }}
{{ . }}
{{- end }}
{{- if .Values.customLabels }}
{{ toYaml .Values.customLabels }}
{{- end }}
{{- end -}}

{{/* matchLabels */}}
{{- define "kyverno.matchLabels" -}}
{{- if .Values.templating.enabled -}}
app: kyverno
{{- end }}
app.kubernetes.io/name: {{ template "kyverno.name" . }}
{{- if not .Values.templating.enabled }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
{{- end -}}

{{/* Create the name of the service to use */}}
{{- define "kyverno.serviceName" -}}
{{- printf "%s-svc" (include "kyverno.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/* Create the name of the service account to use */}}
{{- define "kyverno.serviceAccountName" -}}
{{- if .Values.rbac.serviceAccount.create -}}
    {{ default (include "kyverno.fullname" .) .Values.rbac.serviceAccount.name }}
{{- else -}}
    {{ default "default" .Values.rbac.serviceAccount.name }}
{{- end -}}
{{- end -}}

{{/* Create the default PodDisruptionBudget to use */}}
{{- define "kyverno.podDisruptionBudget.spec" -}}
{{- if and .Values.podDisruptionBudget.minAvailable .Values.podDisruptionBudget.maxUnavailable }}
{{- fail "Cannot set both .Values.podDisruptionBudget.minAvailable and .Values.podDisruptionBudget.maxUnavailable" -}}
{{- end }}
{{- if not .Values.podDisruptionBudget.maxUnavailable }}
minAvailable: {{ default 1 .Values.podDisruptionBudget.minAvailable }}
{{- end }}
{{- if .Values.podDisruptionBudget.maxUnavailable }}
maxUnavailable: {{ .Values.podDisruptionBudget.maxUnavailable }}
{{- end }}
{{- end }}

{{- define "kyverno.securityContext" -}}
{{- if semverCompare "<1.19" .Capabilities.KubeVersion.Version }}
{{ toYaml (omit .Values.securityContext "seccompProfile") }}
{{- else }}
{{ toYaml .Values.securityContext }}
{{- end }}
{{- end }}

{{- define "kyverno.image" -}}
  {{- if .image.registry -}}
{{ .image.registry }}/{{ required "An image repository is required" .image.repository }}:{{ default .defaultTag .image.tag }}
  {{- else -}}
{{ required "An image repository is required" .image.repository }}:{{ default .defaultTag .image.tag }}
  {{- end -}}
{{- end }}
