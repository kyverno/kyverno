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
app.kubernetes.io/version: "{{ .Chart.Version | replace "+" "_" }}"
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
{{- else if has .name .Values.includeRestrictedPolicies }}
{{- true }}
{{- else -}}
{{- false }}
{{- end -}}
{{- end -}}

{{/* Set if a other policies are managed */}}
{{- define "kyverno-policies.podSecurityOther" -}}
{{- if has .name .Values.includeOtherPolicies }}
{{- true }}
{{- else -}}
{{- false }}
{{- end -}}
{{- end -}}

{{/* Set if custom policies are managed */}}
{{- define "kyverno-policies.customPolicies" -}}
    {{- if typeIs "string" .value }}
        {{- tpl .value .context }}
    {{- else }}
        {{- tpl (.value | toYaml) .context }}
    {{- end }}
{{- end -}}

{{/* Get deployed Kyverno version from Kubernetes */}}
{{- define "kyverno-policies.kyvernoVersion" -}}
{{- $version := "" -}}
{{- if eq .Values.kyvernoVersion "autodetect" }}
{{- with (lookup "apps/v1" "Deployment" .Release.Namespace "kyverno") -}}
  {{- with (first .spec.template.spec.containers) -}}
    {{- $imageTag := (last (splitList ":" .image)) -}}
    {{- $version = trimPrefix "v" $imageTag -}}
  {{- end -}}
{{- end -}}
{{ $version }}
{{- else -}}
{{ .Values.kyvernoVersion }}
{{- end -}}
{{- end -}}

{{/* Fail if deployed Kyverno does not match */}}
{{- define "kyverno-policies.supportedKyvernoCheck" -}}
{{- $supportedKyverno := index . "ver" -}}
{{- $top := index . "top" }}
{{- if (include "kyverno-policies.kyvernoVersion" $top) -}}
  {{- if not ( semverCompare $supportedKyverno (include "kyverno-policies.kyvernoVersion" $top) ) -}}
    {{- fail (printf "Kyverno version is too low, expected %s" $supportedKyverno) -}}
  {{- end -}}
{{- end -}}
{{- end -}}

{{/* Custom annotations */}}
{{- define "kyverno-policies.customAnnotations" -}}
{{- with .Values.customAnnotations -}}
{{- toYaml . -}}
{{- end -}}
{{- end -}}
