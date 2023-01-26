{{/* vim: set filetype=mustache: */}}

{{- define "kyverno.reports-controller.name" -}}
{{ template "kyverno.name" . }}-reports-controller
{{- end -}}

{{- define "kyverno.reports-controller.labels" -}}
{{- template "kyverno.labels.merge" (list
  (include "kyverno.labels.common" .)
  (include "kyverno.reports-controller.matchLabels" .)
) -}}
{{- end -}}

{{- define "kyverno.reports-controller.matchLabels" -}}
{{- template "kyverno.labels.merge" (list
  (include "kyverno.matchLabels.common" .)
  (include "kyverno.labels.component" "reports-controller")
) -}}
{{- end -}}

{{- define "kyverno.reports-controller.image" -}}
{{- if .image.registry -}}
  {{ .image.registry }}/{{ required "An image repository is required" .image.repository }}:{{ default .defaultTag .image.tag }}
{{- else -}}
  {{ required "An image repository is required" .image.repository }}:{{ default .defaultTag .image.tag }}
{{- end -}}
{{- end -}}

{{- define "kyverno.reports-controller.roleName" -}}
{{ .Release.Name }}:reports-controller
{{- end -}}

{{/* Create the name of the service account to use */}}
{{- define "kyverno.reports-controller.serviceAccountName" -}}
{{- if .Values.reportsController.rbac.create -}}
    {{ default (include "kyverno.reports-controller.name" .) .Values.reportsController.rbac.serviceAccount.name }}
{{- else -}}
    {{ required "A service account name is required when `rbac.create` is set to `false`" .Values.reportsController.rbac.serviceAccount.name }}
{{- end -}}
{{- end -}}

{{- define "kyverno.reports-controller.securityContext" -}}
{{- if semverCompare "<1.19" .Capabilities.KubeVersion.Version }}
{{ toYaml (omit .Values.reportsController.securityContext "seccompProfile") }}
{{- else }}
{{ toYaml .Values.reportsController.securityContext }}
{{- end }}
{{- end }}

{{/* Create the default PodDisruptionBudget to use */}}
{{- define "kyverno.reports-controller.podDisruptionBudget.spec" -}}
{{- if and .Values.reportsController.podDisruptionBudget.minAvailable .Values.reportsController.podDisruptionBudget.maxUnavailable }}
{{- fail "Cannot set both .Values.reportsController.podDisruptionBudget.minAvailable and .Values.reportsController.podDisruptionBudget.maxUnavailable" -}}
{{- end }}
{{- if not .Values.reportsController.podDisruptionBudget.maxUnavailable }}
minAvailable: {{ default 1 .Values.reportsController.podDisruptionBudget.minAvailable }}
{{- end }}
{{- if .Values.reportsController.podDisruptionBudget.maxUnavailable }}
maxUnavailable: {{ .Values.reportsController.podDisruptionBudget.maxUnavailable }}
{{- end }}
{{- end }}

