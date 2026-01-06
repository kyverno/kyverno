{{/* vim: set filetype=mustache: */}}

{{- define "kyverno.hooks.name" -}}
{{ template "kyverno.name" . }}-hooks
{{- end -}}

{{- define "kyverno.hooks.labels" -}}
{{- template "kyverno.labels.merge" (list
  (include "kyverno.labels.name" (include "kyverno.hooks.name" .))
  (include "kyverno.labels.common" .)
  (include "kyverno.hooks.matchLabels" .)
) -}}
{{- end -}}

{{- define "kyverno.hooks.matchLabels" -}}
{{- template "kyverno.labels.merge" (list
  (include "kyverno.matchLabels.common" .)
  (include "kyverno.labels.component" "hooks")
) -}}
{{- end -}}
