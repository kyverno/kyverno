{{/* vim: set filetype=mustache: */}}

{{- define "kyverno.admission-controller.name" -}}
{{ template "kyverno.name" . }}-admission-controller
{{- end -}}

{{- define "kyverno.admission-controller.labels" -}}
{{- template "kyverno.labels.merge" (list
  (include "kyverno.labels.common" .)
  (include "kyverno.admission-controller.matchLabels" .)
) -}}
{{- end -}}

{{- define "kyverno.admission-controller.matchLabels" -}}
{{- template "kyverno.labels.merge" (list
  (include "kyverno.matchLabels.common" .)
  (include "kyverno.labels.component" "admission-controller")
) -}}
{{- end -}}

{{- define "kyverno.admission-controller.roleName" -}}
{{ .Release.Name }}:admission-controller
{{- end -}}

{{- define "kyverno.admission-controller.serviceAccountName" -}}
{{- if .Values.rbac.serviceAccount.create -}}
    {{ default (include "kyverno.admission-controller.name" .) .Values.rbac.serviceAccount.name }}
{{- else -}}
    {{ default "default" .Values.rbac.serviceAccount.name }}
{{- end -}}
{{- end -}}

{{- define "kyverno.admission-controller.serviceName" -}}
{{- printf "%s-svc" (include "kyverno.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "kyverno.admission-controller.securityContext" -}}
{{- template "kyverno.securityContext" (dict
  "version"         .Capabilities.KubeVersion.Version
  "securityContext" .Values.securityContext
) -}}
{{- end -}}
