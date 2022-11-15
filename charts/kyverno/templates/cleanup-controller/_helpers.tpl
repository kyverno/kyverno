{{/* vim: set filetype=mustache: */}}

{{- define "kyverno.cleanup-controller.deploymentName" -}}
cleanup-controller
{{- end -}}

{{- define "kyverno.cleanup-controller.labels" -}}
app.kubernetes.io/component: cleanup-controller
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/name: {{ template "kyverno.name" . }}
app.kubernetes.io/part-of: {{ template "kyverno.name" . }}
app.kubernetes.io/version: "{{ .Chart.Version }}"
helm.sh/chart: {{ template "kyverno.chart" . }}
{{- end -}}

{{- define "kyverno.cleanup-controller.matchLabels" -}}
app.kubernetes.io/component: cleanup-controller
app.kubernetes.io/name: {{ template "kyverno.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "kyverno.cleanup-controller.image" -}}
{{- if .image.registry -}}
  {{ .image.registry }}/{{ required "An image repository is required" .image.repository }}:{{ default .defaultTag .image.tag }}
{{- else -}}
  {{ required "An image repository is required" .image.repository }}:{{ default .defaultTag .image.tag }}
{{- end -}}
{{- end }}
