{{/* vim: set filetype=mustache: */}}

{{- define "kyverno.chartVersion" -}}
{{- if .Values.templating.enabled -}}
  {{- required "templating.version is required when templating.enabled is true" .Values.templating.version | replace "+" "_" -}}
{{- else -}}
  {{- .Chart.Version | replace "+" "_" -}}
{{- end -}}
{{- end -}}

{{- define "kyverno.features.flags" -}}
{{- $flags := list -}}
{{- with .admissionReports -}}
  {{- $flags = append $flags (print "--admissionReports=" .enabled) -}}
{{- end -}}
{{- with .autoUpdateWebhooks -}}
  {{- $flags = append $flags (print "--autoUpdateWebhooks=" .enabled) -}}
{{- end -}}
{{- with .backgroundScan -}}
  {{- $flags = append $flags (print "--backgroundScan=" .enabled) -}}
  {{- $flags = append $flags (print "--backgroundScanWorkers=" .backgroundScanWorkers) -}}
  {{- $flags = append $flags (print "--backgroundScanInterval=" .backgroundScanInterval) -}}
  {{- $flags = append $flags (print "--skipResourceFilters=" .skipResourceFilters) -}}
{{- end -}}
{{- with .configMapCaching -}}
  {{- $flags = append $flags (print "--enableConfigMapCaching=" .enabled) -}}
{{- end -}}
{{- with .dumpPayload -}}
  {{- $flags = append $flags (print "--dumpPayload=" .enabled) -}}
{{- end -}}
{{- with .forceFailurePolicyIgnore -}}
  {{- $flags = append $flags (print "--forceFailurePolicyIgnore=" .enabled) -}}
{{- end -}}
{{- with .policyExceptions -}}
  {{- $flags = append $flags (print "--enablePolicyException=" .enabled) -}}
  {{- $flags = append $flags (print "--exceptionNamespace=" (.namespace | quote)) -}}
{{- end -}}
{{- with .protectManagedResources -}}
  {{- $flags = append $flags (print "--protectManagedResources=" .enabled) -}}
{{- end -}}
{{- with .reports -}}
  {{- $flags = append $flags (print "--reportsChunkSize=" .chunkSize) -}}
{{- end -}}
{{- with $flags -}}
  {{- toYaml . -}}
{{- end -}}
{{- end -}}
