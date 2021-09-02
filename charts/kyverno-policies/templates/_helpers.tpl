{{/* vim: set filetype=mustache: */}}

{{/* Set if a baseline policy is managed */}}
{{- define "kyverno.podSecurityBaseline" -}}
{{- if or (eq .Values.podSecurityStandard "baseline") (eq .Values.podSecurityStandard "restricted") }}
{{- true }}
{{- else if and (eq .Values.podSecurityStandard "custom") (has .name .Values.podSecurityPolicies) }}
{{- true }}
{{- else -}}
{{- false }}
{{- end -}}
{{- end -}}

{{/* Set if a restricted policy is managed */}}
{{- define "kyverno.podSecurityRestricted" -}}
{{- if eq .Values.podSecurityStandard "restricted" }}
{{- true }}
{{- else if and (eq .Values.podSecurityStandard "custom") (has .name .Values.podSecurityPolicies) }}
{{- true }}
{{- else -}}
{{- false }}
{{- end -}}
{{- end -}}
