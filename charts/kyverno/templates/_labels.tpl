{{/* vim: set filetype=mustache: */}}

{{- define "kyverno.utils.helmLabels" -}}
{{- if not .Values.templating.enabled -}}
helm.sh/chart: {{ template "kyverno.chart" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}
{{- end -}}

{{- define "kyverno.utils.versionLabels" -}}
app.kubernetes.io/version: {{ template "kyverno.chartVersion" . }}
{{- end -}}

{{- define "kyverno.utils.commonLabels" -}}
app.kubernetes.io/part-of: {{ template "kyverno.name" . }}
{{- with (include "kyverno.helmLabels" .)     -}}{{- . | trim | nindent 0 -}}{{- end -}}
{{- with (include "kyverno.versionLabels" .)  -}}{{- . | trim | nindent 0 -}}{{- end -}}
{{- end -}}

{{- define "kyverno.admission-controller.labels" -}}
{{- with (include "kyverno.utils.commonLabels" .) -}}{{- . | trim | nindent 0 -}}{{- end -}}
{{- with (include "kyverno.matchLabels" .)        -}}{{- . | trim | nindent 0 -}}{{- end -}}
{{- if .Values.customLabels }}
{{ toYaml .Values.customLabels }}
{{- end }}
{{- end -}}

{{- define "kyverno.admission-controller.matchLabels" -}}
app.kubernetes.io/component: admission-controller
app.kubernetes.io/name: {{ template "kyverno.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}
