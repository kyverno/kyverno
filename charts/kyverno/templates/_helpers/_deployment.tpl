{{/* vim: set filetype=mustache: */}}

{{- define "kyverno.deployment.replicas" -}}
  {{- if and (kindIs "int" .)  (eq (int (default 1 .)) 0) -}}
    {{- fail "Kyverno does not support running with 0 replicas. Please provide a non-zero integer value." -}}
  {{- end -}}
  {{- . -}}
{{- end -}}
