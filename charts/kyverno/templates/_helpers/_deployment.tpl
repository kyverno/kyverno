{{/* vim: set filetype=mustache: */}}

{{- define "kyverno.deployment.replicas" -}}
  {{- if eq . 0 -}}
    {{- fail "Kyverno does not support running with 0 replicas. Please provide a non-zero value." -}}
  {{- end -}}
  {{- . -}}
{{- end -}}