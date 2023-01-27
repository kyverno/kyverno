{{/* vim: set filetype=mustache: */}}

{{- define "kyverno.image" -}}
{{- if .image.registry -}}
  {{- printf "%s/%s:%s" .image.registry (required "An image repository is required" .image.repository) (default .defaultTag .image.tag) -}}
{{- else -}}
  {{- printf "%s:%s" (required "An image repository is required" .image.repository) (default .defaultTag .image.tag) -}}
{{- end -}}
{{- end -}}
