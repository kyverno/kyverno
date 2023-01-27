{{/* vim: set filetype=mustache: */}}

{{- define "kyverno.securityContext" -}}
{{- toYaml . -}}
{{- end -}}
