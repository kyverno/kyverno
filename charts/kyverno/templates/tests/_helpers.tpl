{{/* vim: set filetype=mustache: */}}

{{- define "kyverno.test.labels" -}}
app.kubernetes.io/part-of: {{ template "kyverno.name" . }}
app.kubernetes.io/component: test
{{- with (include "kyverno.helmLabels" .)     -}}{{- . | trim | nindent 0 -}}{{- end -}}
{{- with (include "kyverno.matchLabels" .)    -}}{{- . | trim | nindent 0 -}}{{- end -}}
{{- with (include "kyverno.versionLabels" .)  -}}{{- . | trim | nindent 0 -}}{{- end -}}
{{- end -}}

{{- define "kyverno.test.securityContext" -}}
{{- if .Values.test.securityContext -}}
  {{- if semverCompare "<1.19" .Capabilities.KubeVersion.Version -}}
    {{ toYaml (omit .Values.test.securityContext "seccompProfile") }}
  {{- else -}}
    {{ toYaml .Values.test.securityContext }}
  {{- end -}}
{{- end -}}
{{- end -}}
