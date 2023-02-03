{{/* vim: set filetype=mustache: */}}

{{- define "replicaCountCheck" }}
  {{- $value := . }}
  {{- if eq $value 0 }}
    {{ fail "Kyverno does not support running with 0 replicas. Please provide a non-zero value." }}
  {{- end }}
{{- end }}