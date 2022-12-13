{{/* vim: set filetype=mustache: */}}

{{- define "kyverno.cleanup-controller.name" -}}
{{ template "kyverno.name" . }}-cleanup-controller
{{- end -}}

{{- define "kyverno.cleanup-controller.labels" -}}
{{- include "kyverno.cleanup-controller.matchLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: {{ template "kyverno.name" . }}
app.kubernetes.io/version: "{{ .Chart.Version | replace "+" "_" }}"
helm.sh/chart: {{ template "kyverno.chart" . }}
{{- end -}}

{{- define "kyverno.cleanup-controller.matchLabels" -}}
app.kubernetes.io/component: cleanup-controller
app.kubernetes.io/name: {{ template "kyverno.cleanup-controller.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "kyverno.cleanup-controller.image" -}}
{{- if .image.registry -}}
  {{ .image.registry }}/{{ required "An image repository is required" .image.repository }}:{{ default .defaultTag .image.tag }}
{{- else -}}
  {{ required "An image repository is required" .image.repository }}:{{ default .defaultTag .image.tag }}
{{- end -}}
{{- end -}}

{{/* Create the name of the service account to use */}}
{{- define "kyverno.cleanup-controller.serviceAccountName" -}}
{{- if .Values.cleanupController.rbac.create -}}
    {{ default (include "kyverno.cleanup-controller.name" .) .Values.cleanupController.rbac.serviceAccount.name }}
{{- else -}}
    {{ required "A service account name is required when `rbac.create` is set to `false`" .Values.cleanupController.rbac.serviceAccount.name }}
{{- end -}}
{{- end -}}

{{- define "kyverno.cleanup-controller.securityContext" -}}
{{- if semverCompare "<1.19" .Capabilities.KubeVersion.Version }}
{{ toYaml (omit .Values.cleanupController.securityContext "seccompProfile") }}
{{- else }}
{{ toYaml .Values.cleanupController.securityContext }}
{{- end }}
{{- end }}

{{/* Create the default PodDisruptionBudget to use */}}
{{- define "kyverno.cleanup-controller.podDisruptionBudget.spec" -}}
{{- if and .Values.cleanupController.podDisruptionBudget.minAvailable .Values.cleanupController.podDisruptionBudget.maxUnavailable }}
{{- fail "Cannot set both .Values.cleanupController.podDisruptionBudget.minAvailable and .Values.cleanupController.podDisruptionBudget.maxUnavailable" -}}
{{- end }}
{{- if not .Values.cleanupController.podDisruptionBudget.maxUnavailable }}
minAvailable: {{ default 1 .Values.cleanupController.podDisruptionBudget.minAvailable }}
{{- end }}
{{- if .Values.cleanupController.podDisruptionBudget.maxUnavailable }}
maxUnavailable: {{ .Values.cleanupController.podDisruptionBudget.maxUnavailable }}
{{- end }}
{{- end }}

