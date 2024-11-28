{{/* vim: set filetype=mustache: */}}

{{- define "kyverno.rbac.labels.admin" -}}
{{- if .Values.rbac.aggregateClusterRoles -}}
{{- template "kyverno.labels.merge" (list
  (include "kyverno.labels.common" .)
  (include "kyverno.rbac.matchLabels" .)
  "rbac.authorization.k8s.io/aggregate-to-admin: 'true'"
) -}}
{{- end -}}
{{- end -}}

{{- define "kyverno.rbac.labels.view" -}}
{{- if .Values.rbac.aggregateClusterRoles -}}
{{- template "kyverno.labels.merge" (list
  (include "kyverno.labels.common" .)
  (include "kyverno.rbac.matchLabels" .)
  "rbac.authorization.k8s.io/aggregate-to-view: 'true'"
) -}}
{{- end -}}
{{- end -}}

{{- define "kyverno.rbac.matchLabels" -}}
{{- template "kyverno.labels.merge" (list
  (include "kyverno.matchLabels.common" .)
  (include "kyverno.labels.component" "rbac")
) -}}
{{- end -}}

{{- define "kyverno.rbac.roleName" -}}
{{ include "kyverno.fullname" . }}:rbac
{{- end -}}
