{{/*
Reports Server helpers
*/}}

{{/*
Check if reports-server is enabled
*/}}
{{- define "kyverno.reportsServer.enabled" -}}
{{- if .Values.reportsServer.enabled -}}
true
{{- else -}}
false
{{- end -}}
{{- end -}}

{{/*
Reports Server readiness init container
*/}}
{{- define "kyverno.reportsServer.initContainer" -}}
{{- if and .Values.reportsServer.enabled .Values.reportsServer.waitForReady }}
- name: wait-for-reports-server
  image: {{ include "kyverno.image" (dict "globalRegistry" .Values.global.image.registry "image" .Values.test.image "defaultTag" .Values.test.image.tag) | quote }}
  imagePullPolicy: {{ .Values.test.image.pullPolicy | default "IfNotPresent" }}
  args:
    - check-endpoints
    - --service-name={{ .Release.Name }}-reports-server
    - --namespace={{ .Release.Namespace }}
    - --timeout={{ .Values.reportsServer.readinessTimeout }}
  securityContext:
    runAsUser: 65534
    runAsGroup: 65534
    runAsNonRoot: true
    privileged: false
    allowPrivilegeEscalation: false
    readOnlyRootFilesystem: true
    capabilities:
      drop:
        - ALL
    seccompProfile:
      type: RuntimeDefault
  resources:
    limits:
      cpu: 100m
      memory: 128Mi
    requests:
      cpu: 10m
      memory: 32Mi
{{- end }}
{{- end }}

{{/*
Reports Server service dependency annotation
*/}}
{{- define "kyverno.reportsServer.dependsOnAnnotation" -}}
{{- if .Values.reportsServer.enabled }}
"helm.sh/hook-depends-on": "Service/reports-server"
{{- end -}}
{{- end -}}
