{{/* vim: set filetype=mustache: */}}

{{- define "kyverno.crds.labels" -}}
app.kubernetes.io/part-of: {{ template "kyverno.name" . }}
{{- with (include "kyverno.helmLabels" .) }}
{{ . }}
{{- end }}
{{- with (include "kyverno.matchLabels" .) }}
{{ . }}
{{- end }}
{{- with (include "kyverno.versionLabels" .) }}
{{ . }}
{{- end }}
{{- end -}}
