{{/* vim: set filetype=mustache: */}}

{{- define "kyverno.config.configMapName" -}}
{{- if .Values.config.create -}}
    {{ default (include "kyverno.fullname" .) .Values.config.name }}
{{- else -}}
    {{ required "A configmap name is required when `config.create` is set to `false`" .Values.config.name }}
{{- end -}}
{{- end -}}

{{- define "kyverno.config.metricsConfigMapName" -}}
{{- if .Values.metricsConfig.create -}}
    {{ default (printf "%s-metrics" (include "kyverno.fullname" .)) .Values.metricsConfig.name }}
{{- else -}}
    {{ required "A configmap name is required when `metricsConfig.create` is set to `false`" .Values.metricsConfig.name }}
{{- end -}}
{{- end -}}

{{- define "kyverno.config.labels" -}}
app.kubernetes.io/part-of: {{ template "kyverno.name" . }}
{{- with (include "kyverno.helmLabels" .)     -}}{{- . | trim | nindent 0 -}}{{- end -}}
{{- with (include "kyverno.matchLabels" .)    -}}{{- . | trim | nindent 0 -}}{{- end -}}
{{- with (include "kyverno.versionLabels" .)  -}}{{- . | trim | nindent 0 -}}{{- end -}}
{{- if .Values.customLabels }}
{{ toYaml .Values.customLabels }}
{{- end }}
{{- end -}}

{{- define "kyverno.config.resourceFilters" -}}
{{- $resourceFilters := .Values.config.resourceFilters }}
{{- if .Values.excludeKyvernoNamespace }}
  {{- $resourceFilters = prepend .Values.config.resourceFilters (printf "[*,%s,*]" (include "kyverno.namespace" .)) }}
{{- end }}
{{- range $exclude := .Values.resourceFiltersExcludeNamespaces }}
  {{- range $filter := $resourceFilters }}
    {{- if (contains (printf ",%s," $exclude) $filter) }}
      {{- $resourceFilters = without $resourceFilters $filter }}
    {{- end }}
  {{- end }}
{{- end }}
{{- tpl (join "" $resourceFilters) . }}
{{- end }}

{{- define "kyverno.config.webhooks" -}}
{{- $excludeDefault := dict "key" "kubernetes.io/metadata.name" "operator" "NotIn" "values" (list (include "kyverno.namespace" .)) }}
{{- $newWebhook := list }}
{{- range $webhook := .Values.config.webhooks }}
  {{- $namespaceSelector := default dict $webhook.namespaceSelector }}
  {{- $matchExpressions := default list $namespaceSelector.matchExpressions }}
  {{- $newNamespaceSelector := dict "matchLabels" $namespaceSelector.matchLabels "matchExpressions" (append $matchExpressions $excludeDefault) }}
  {{- $newWebhook = append $newWebhook (merge (omit $webhook "namespaceSelector") (dict "namespaceSelector" $newNamespaceSelector)) }}
{{- end }}
{{- $newWebhook | toJson }}
{{- end }}
