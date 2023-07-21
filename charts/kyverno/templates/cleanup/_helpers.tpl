{{/* vim: set filetype=mustache: */}}

{{- define "kyverno.cleanup.labels" -}}
{{- template "kyverno.labels.merge" (list
  (include "kyverno.labels.common" .)
  (include "kyverno.matchLabels.common" .)
  (include "kyverno.labels.component" "cleanup")
) -}}
{{- end -}}
