{{/* vim: set filetype=mustache: */}}

{{- define "kyverno.securityContext" -}}
{{- if semverCompare "<1.19" .version -}}
  {{- toYaml (omit .securityContext "seccompProfile") -}}
{{- else -}}
  {{- toYaml .securityContext -}}
{{- end -}}
{{- end -}}
