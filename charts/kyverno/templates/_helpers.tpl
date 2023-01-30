{{/* vim: set filetype=mustache: */}}

{{- define "kyverno.chartVersion" -}}
{{- if .Values.templating.enabled -}}
  {{- required "templating.version is required when templating.enabled is true" .Values.templating.version | replace "+" "_" -}}
{{- else -}}
  {{- .Chart.Version | replace "+" "_" -}}
{{- end -}}
{{- end -}}
