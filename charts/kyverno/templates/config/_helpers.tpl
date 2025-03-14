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
{{- template "kyverno.labels.merge" (list
  (include "kyverno.labels.common" .)
  (include "kyverno.config.matchLabels" .)
) -}}
{{- end -}}

{{- define "kyverno.config.matchLabels" -}}
{{- template "kyverno.labels.merge" (list
  (include "kyverno.matchLabels.common" .)
  (include "kyverno.labels.component" "config")
) -}}
{{- end -}}

{{- define "kyverno.config.resourceFilters" -}}
{{- $resourceFilters := .Values.config.resourceFilters -}}
{{- if .Values.config.excludeKyvernoNamespace -}}
  {{- $resourceFilters = prepend .Values.config.resourceFilters (printf "[*/*,%s,*]" (include "kyverno.namespace" .)) -}}
{{- end -}}
{{- range $resourceExclude := .Values.config.resourceFiltersExclude -}}
  {{- $resourceFilters = without $resourceFilters $resourceExclude -}}
{{- end -}}
{{- range $exclude := .Values.config.resourceFiltersExcludeNamespaces -}}
  {{- range $filter := $resourceFilters -}}
    {{- if (contains (printf ",%s," $exclude) $filter) -}}
      {{- $resourceFilters = without $resourceFilters $filter -}}
    {{- end -}}
  {{- end -}}
{{- end -}}
{{- $resourceFilters = concat $resourceFilters .Values.config.resourceFiltersInclude -}}
{{- range $include := .Values.config.resourceFiltersIncludeNamespaces -}}
  {{- $resourceFilters = append $resourceFilters (printf "[*/*,%s,*]" $include) -}}
{{- end -}}
{{- range $resourceFilter := $resourceFilters }}
{{ tpl $resourceFilter $ }}
{{- end -}}
{{- end -}}

{{- define "kyverno.config.webhooks" -}}
{{- /* Create a list of namespaces to exclude */}}
{{- $excludedNamespaces := list }}

{{- /* Always add the Kyverno namespace if exclusion is enabled */}}
{{- if .Values.config.excludeKyvernoNamespace }}
  {{- $excludedNamespaces = append $excludedNamespaces (include "kyverno.namespace" .) }}
{{- end }}

{{- /* Add kube-system namespace to be excluded if defined in values */}}
{{- if .Values.config.webhooks }}
  {{- if .Values.config.webhooks.namespaceSelector }}
    {{- if .Values.config.webhooks.namespaceSelector.matchExpressions }}
      {{- range .Values.config.webhooks.namespaceSelector.matchExpressions }}
        {{- if (and (eq .operator "NotIn") (eq .key "kubernetes.io/metadata.name")) }}
          {{- range .values }}
            {{- $excludedNamespaces = append $excludedNamespaces . }}
          {{- end }}
        {{- end }}
      {{- end }}
    {{- end }}
  {{- end }}
{{- end }}

{{- /* Always include kube-system if not already in the list */}}
{{- if not (has "kube-system" $excludedNamespaces) }}
  {{- $excludedNamespaces = append $excludedNamespaces "kube-system" }}
{{- end }}

{{- /* Ensure the excluded namespaces list is unique */}}
{{- $excludedNamespaces = uniq $excludedNamespaces }}

{{- $excludeDefault := dict "key" "kubernetes.io/metadata.name" "operator" "NotIn" "values" $excludedNamespaces }}
{{- $webhooks := .Values.config.webhooks -}}
{{- if $webhooks | typeIs "slice" -}}
  {{- $newWebhooks := dict -}}
  {{- range $index, $webhook := $webhooks -}}
    {{- if $webhook.namespaceSelector -}}
      {{- $namespaceSelector := $webhook.namespaceSelector }}
      {{- $matchExpressions := default (list) $namespaceSelector.matchExpressions }}
      {{- $newNamespaceSelector := dict "matchLabels" $namespaceSelector.matchLabels "matchExpressions" (list $excludeDefault) }}
      {{- $newWebhook := merge (omit $webhook "namespaceSelector") (dict "namespaceSelector" $newNamespaceSelector) }}
      {{- $newWebhooks = merge $newWebhooks (dict $webhook.name $newWebhook) }}
    {{- end -}}
  {{- end -}}
  {{- $newWebhooks | toJson }}
{{- else -}}
  {{- $webhook := $webhooks }}
  {{- $namespaceSelector := default (dict) $webhook.namespaceSelector }}
  {{- $matchExpressions := default (list) $namespaceSelector.matchExpressions }}
  {{- $newNamespaceSelector := dict "matchLabels" $namespaceSelector.matchLabels "matchExpressions" (list $excludeDefault) }}
  {{- $newWebhook := merge (omit $webhook "namespaceSelector") (dict "namespaceSelector" $newNamespaceSelector) }}
  {{- $newWebhook | toJson }}
{{- end -}}
{{- end -}}

{{- define "kyverno.config.imagePullSecret" -}}
{{- printf "{\"auths\":{\"%s\":{\"auth\":\"%s\"}}}" .registry (printf "%s:%s" .username .password | b64enc) | b64enc }}
{{- end -}}
