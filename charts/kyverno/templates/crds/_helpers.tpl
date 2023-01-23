{{/* vim: set filetype=mustache: */}}

{{- define "kyverno.crd.labels" -}}
{{- with (include "kyverno.utils.commonLabels" .) -}}{{- . | trim | nindent 0 -}}{{- end -}}
{{- with (include "kyverno.matchLabels" .)        -}}{{- . | trim | nindent 0 -}}{{- end -}}
{{- end -}}
