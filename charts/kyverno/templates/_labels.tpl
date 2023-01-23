{{/* vim: set filetype=mustache: */}}

{{- define "kyverno.labels.helm" -}}
{{- if not .Values.templating.enabled -}}
helm.sh/chart: {{ template "kyverno.chart" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}
{{- end -}}

{{- define "kyverno.labels.version" -}}
app.kubernetes.io/version: {{ template "kyverno.chartVersion" . }}
{{- end -}}

{{- define "kyverno.labels.common" -}}
app.kubernetes.io/part-of: {{ template "kyverno.fullname" . }}
app.kubernetes.io/name: {{ template "kyverno.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- with (include "kyverno.labels.helm" .)     -}}{{- . | trim | nindent 0 -}}{{- end -}}
{{- with (include "kyverno.labels.version" .)  -}}{{- . | trim | nindent 0 -}}{{- end -}}
{{- end -}}

{{- define "kyverno.admission-controller.labels" -}}
{{- with (include "kyverno.labels.common" .)               -}}{{- . | trim | nindent 0 -}}{{- end -}}
{{- with (include "kyverno.admission-controller.matchLabels" .) -}}{{- . | trim | nindent 0 -}}{{- end -}}
{{- end -}}

{{- define "kyverno.admission-controller.matchLabels" -}}
app.kubernetes.io/component: admission-controller
app.kubernetes.io/name: {{ template "kyverno.name" . }}
{{- end -}}

{{- define "kyverno.labels" -}}
{{- end -}}

{{- define "kyverno.matchLabels" -}}
{{- end -}}
