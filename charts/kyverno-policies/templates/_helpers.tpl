{{/* vim: set filetype=mustache: */}}
{{/* Expand the name of the chart. */}}
{{- define "kyverno-policies.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/* Create chart name and version as used by the chart label. */}}
{{- define "kyverno-policies.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/* Helm required labels */}}
{{- define "kyverno-policies.labels" -}}
app.kubernetes.io/component: kyverno
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/name: {{ template "kyverno-policies.name" . }}
app.kubernetes.io/part-of: {{ template "kyverno-policies.name" . }}
app.kubernetes.io/version: "{{ .Chart.Version }}"
helm.sh/chart: {{ template "kyverno-policies.chart" . }}
{{- if .Values.customLabels }}
{{ toYaml .Values.customLabels }}
{{- end }}
{{- end -}}

{{/* Set if a baseline policy is managed */}}
{{- define "kyverno-policies.podSecurityBaseline" -}}
{{- if or (eq .Values.podSecurityStandard "baseline") (eq .Values.podSecurityStandard "restricted") }}
{{- true }}
{{- else if and (eq .Values.podSecurityStandard "custom") (has .name .Values.podSecurityPolicies) }}
{{- true }}
{{- else -}}
{{- false }}
{{- end -}}
{{- end -}}

{{/* Set if a restricted policy is managed */}}
{{- define "kyverno-policies.podSecurityRestricted" -}}
{{- if eq .Values.podSecurityStandard "restricted" }}
{{- true }}
{{- else if and (eq .Values.podSecurityStandard "custom") (has .name .Values.podSecurityPolicies) }}
{{- true }}
{{- else -}}
{{- false }}
{{- end -}}
{{- end -}}
