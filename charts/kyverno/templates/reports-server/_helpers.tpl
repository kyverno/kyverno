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
  command:
    - /bin/sh
    - -c
    - |
      echo "Waiting for reports-server to be ready..."
      timeout={{ .Values.reportsServer.readinessTimeout | default 300 }}
      elapsed=0
      
      # Check if kubectl is available
      if ! command -v kubectl >/dev/null 2>&1; then
        echo "kubectl not available in this image, using basic readiness check"
        while [ $elapsed -lt $timeout ]; do
          if nc -z {{ .Release.Name }}-reports-server.{{ include "kyverno.namespace" . }}.svc.cluster.local 443 2>/dev/null; then
            echo "Reports-server is responding on port 443!"
            exit 0
          fi
          echo "Waiting for reports-server... ($elapsed/$timeout seconds)"
          sleep 5
          elapsed=$((elapsed + 5))
        done
      else
        # Use kubectl for more sophisticated checks
        while [ $elapsed -lt $timeout ]; do
          if kubectl get endpoints {{ .Release.Name }}-reports-server -n {{ include "kyverno.namespace" . }} > /dev/null 2>&1; then
            if kubectl get endpoints {{ .Release.Name }}-reports-server -n {{ include "kyverno.namespace" . }} -o jsonpath='{.subsets[*].addresses[*].ip}' | grep -q .; then
              echo "Reports-server is ready!"
              exit 0
            fi
          fi
          echo "Waiting for reports-server... ($elapsed/$timeout seconds)"
          sleep 5
          elapsed=$((elapsed + 5))
        done
      fi
      
      echo "Timeout waiting for reports-server to be ready"
      exit 1
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