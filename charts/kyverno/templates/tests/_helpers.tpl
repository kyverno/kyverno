{{/* vim: set filetype=mustache: */}}

{{- define "kyverno.test.labels" -}}
{{- template "kyverno.labels.merge" (list
  (include "kyverno.labels.common" .)
  (include "kyverno.test.matchLabels" .)
) -}}
{{- end -}}

{{- define "kyverno.test.matchLabels" -}}
{{- template "kyverno.labels.merge" (list
  (include "kyverno.matchLabels.common" .)
  (include "kyverno.labels.component" "test")
) -}}
{{- end -}}

{{- define "kyverno.test.annotations" -}}
helm.sh/hook: test
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

{{- define "kyverno.test.image" -}}
{{- template "kyverno.image" (dict "image" .Values.test.image "defaultTag" "latest") -}}
{{- end -}}

{{- define "kyverno.test.imagePullPolicy" -}}
{{- default .Values.image.pullPolicy .Values.test.image.pullPolicy -}}
{{- end -}}
