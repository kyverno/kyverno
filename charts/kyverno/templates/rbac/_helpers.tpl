{{/* vim: set filetype=mustache: */}}

{{- define "kyverno.rbac.labels.admin" -}}
{{- $labels := list
  (include "kyverno.labels.common" .)
  (include "kyverno.rbac.matchLabels" .)
-}}
{{- if .Values.rbac.roles.aggregate.admin -}}
{{- $labels = append $labels "rbac.authorization.k8s.io/aggregate-to-admin: 'true'" -}}
{{- end -}}
{{- template "kyverno.labels.merge" $labels -}}
{{- end -}}


{{- define "kyverno.rbac.labels.view" -}}
{{- $labels := list
  (include "kyverno.labels.common" .)
  (include "kyverno.rbac.matchLabels" .)
-}}
{{- if .Values.rbac.roles.aggregate.view -}}
{{- $labels = append $labels "rbac.authorization.k8s.io/aggregate-to-view: 'true'" -}}
{{- end -}}
{{- template "kyverno.labels.merge" $labels -}}
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
