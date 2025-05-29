{{/* vim: set filetype=mustache: */}}

{{- define "kyverno.pdb.apiVersion" -}}
{{- if .Values.apiVersionOverride.podDisruptionBudget -}}
  {{- .Values.apiVersionOverride.podDisruptionBudget -}}
{{- else -}}
  policy/v1
{{- end -}}
{{- end -}}

{{- define "kyverno.pdb.spec" -}}
{{- if and .minAvailable .maxUnavailable -}}
  {{- fail "Cannot set both .minAvailable and .maxUnavailable" -}}
{{- end -}}
{{- if not .maxUnavailable }}
minAvailable: {{ default 1 .minAvailable }}
{{- end }}
{{- if .maxUnavailable }}
maxUnavailable: {{ .maxUnavailable }}
{{- end }}
{{- if .unhealthyPodEvictionPolicy }}
unhealthyPodEvictionPolicy: {{ .unhealthyPodEvictionPolicy }}
{{- end }}
{{- end -}}
