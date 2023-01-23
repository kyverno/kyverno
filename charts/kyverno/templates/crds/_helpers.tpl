{{/* vim: set filetype=mustache: */}}

{{- define "kyverno.crd.labels" -}}
{{- with (include "kyverno.labels.common" .) -}}{{- . | trim | nindent 0 -}}{{- end -}}
{{- end -}}
