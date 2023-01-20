{{/* vim: set filetype=mustache: */}}

{{- define "kyverno.crd.labels" -}}
app.kubernetes.io/part-of: {{ template "kyverno.name" . }}
{{- with (include "kyverno.helmLabels" .)     -}}{{- . | trim | nindent 0 -}}{{- end -}}
{{- with (include "kyverno.matchLabels" .)    -}}{{- . | trim | nindent 0 -}}{{- end -}}
{{- with (include "kyverno.versionLabels" .)  -}}{{- . | trim | nindent 0 -}}{{- end -}}
{{- end -}}
