{{/* vim: set filetype=mustache: */}}
{{/* Expand the name of the chart. */}}
{{- define "kyverno-policies.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/* Create chart name and version as used by the chart label. */}}
{{- define "kyverno-policies.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/* Helm required labels */}}
{{- define "kyverno-policies.labels" -}}
app.kubernetes.io/component: kyverno
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/name: {{ template "kyverno-policies.name" . }}
app.kubernetes.io/part-of: {{ template "kyverno-policies.name" . }}
app.kubernetes.io/version: "{{ .Chart.Version | replace "+" "_" }}"
helm.sh/chart: {{ template "kyverno-policies.chart" . }}
{{- if .Values.customLabels }}
{{ toYaml .Values.customLabels }}
{{- end }}
{{- end -}}

{{/* Set if a baseline policy is managed */}}
{{- define "kyverno-policies.podSecurityBaseline" -}}
{{- if or (eq .Values.podSecurityStandard "baseline") (eq .Values.podSecurityStandard "restricted") }}
{{- true }}
{{- else if and (eq .Values.podSecurityStandard "custom") (has .name .Values.podSecurityPolicies) }}
{{- true }}
{{- else -}}
{{- false }}
{{- end -}}
{{- end -}}

{{/* Set if a restricted policy is managed */}}
{{- define "kyverno-policies.podSecurityRestricted" -}}
{{- if eq .Values.podSecurityStandard "restricted" }}
{{- true }}
{{- else if and (eq .Values.podSecurityStandard "custom") (has .name .Values.podSecurityPolicies) }}
{{- true }}
{{- else if has .name .Values.includeRestrictedPolicies }}
{{- true }}
{{- else -}}
{{- false }}
{{- end -}}
{{- end -}}

{{/* Set if a other policies are managed */}}
{{- define "kyverno-policies.podSecurityOther" -}}
{{- if has .name .Values.includeOtherPolicies }}
{{- true }}
{{- else -}}
{{- false }}
{{- end -}}
{{- end -}}

{{/* Set if custom policies are managed */}}
{{- define "kyverno-policies.customPolicies" -}}
    {{- if typeIs "string" .value }}
        {{- tpl .value .context }}
    {{- else }}
        {{- tpl (.value | toYaml) .context }}
    {{- end }}
{{- end -}}

{{/* Get deployed Kyverno version from Kubernetes */}}
{{- define "kyverno-policies.kyvernoVersion" -}}
{{- $version := "" -}}
{{- if eq .Values.kyvernoVersion "autodetect" }}
{{- with (lookup "apps/v1" "Deployment" .Release.Namespace "kyverno") -}}
  {{- with (first .spec.template.spec.containers) -}}
    {{- $imageTag := (last (splitList ":" .image)) -}}
    {{- $version = trimPrefix "v" $imageTag -}}
  {{- end -}}
{{- end -}}
{{ $version }}
{{- else -}}
{{ .Values.kyvernoVersion }}
{{- end -}}
{{- end -}}

{{/* Fail if deployed Kyverno does not match */}}
{{- define "kyverno-policies.supportedKyvernoCheck" -}}
{{- $supportedKyverno := index . "ver" -}}
{{- $top := index . "top" }}
{{- if (include "kyverno-policies.kyvernoVersion" $top) -}}
  {{- if not ( semverCompare $supportedKyverno (include "kyverno-policies.kyvernoVersion" $top) ) -}}
    {{- fail (printf "Kyverno version is too low, expected %s" $supportedKyverno) -}}
  {{- end -}}
{{- end -}}
{{- end -}}

{{/* Custom annotations */}}
{{- define "kyverno-policies.customAnnotations" -}}
{{- with .Values.customAnnotations -}}
{{- toYaml . -}}
{{- end -}}
{{- end -}}

{{/* Check if ValidatingPolicy should be used */}}
{{- define "kyverno-policies.useValidatingPolicy" -}}
{{- if eq .Values.policyType "ValidatingPolicy" -}}
{{- true -}}
{{- else -}}
{{- false -}}
{{- end -}}
{{- end -}}

{{/* Get validationActions from validationFailureAction for ValidatingPolicy */}}
{{/* Maps: Audit -> Audit, Enforce -> Deny */}}
{{- define "kyverno-policies.validationActions" -}}
{{- if eq . "Enforce" -}}
{{- list "Deny" | toYaml -}}
{{- else -}}
{{- list "Audit" | toYaml -}}
{{- end -}}
{{- end -}}

{{/* Get validationActions for a specific policy */}}
{{- define "kyverno-policies.policyValidationActions" -}}
{{- $policyName := index . "name" -}}
{{- $values := index . "values" -}}
{{- $defaultAction := $values.validationFailureAction -}}
{{- $policyAction := index $values.validationFailureActionByPolicy $policyName -}}
{{- $action := default $defaultAction $policyAction -}}
{{- include "kyverno-policies.validationActions" $action -}}
{{- end -}}

{{/* Resolve vpolExclude for a specific policy.
     Per-policy entry (vpolExcludeByPolicy.<name>) replaces the global default (vpolExclude) entirely.
     Receives dict with "name" (policy name) and "values" (.Values).
     Returns the resolved exclude dict (may be empty). */}}
{{- define "kyverno-policies.policyVpolExclude" -}}
{{- $policyName := index . "name" -}}
{{- $values := index . "values" -}}
{{- $perPolicy := index $values.vpolExcludeByPolicy $policyName | default dict -}}
{{- if gt (len $perPolicy) 0 -}}
  {{- toYaml $perPolicy -}}
{{- else -}}
  {{- $global := $values.vpolExclude | default dict -}}
  {{- if gt (len $global) 0 -}}
    {{- toYaml $global -}}
  {{- end -}}
{{- end -}}
{{- end -}}

{{/* Generate matchConditions from a vpolExclude entry dict.
     Receives the per-policy exclude dict (e.g. .excludeNamespaces, .excludeSubjects, .matchConditions).
     Only emits the block when at least one condition is generated. */}}
{{- define "kyverno-policies.vpolExcludeMatchConditions" -}}
{{- $conditions := list -}}
{{- $namespaces := .excludeNamespaces | default list -}}
{{- if gt (len $namespaces) 0 -}}
  {{- $conditions = append $conditions (dict "name" "exclude-namespaces" "expression" (printf "!(object.metadata.namespace in [%s])" (include "kyverno-policies.celStringList" $namespaces))) -}}
{{- end -}}
{{- $subjects := .excludeSubjects | default list -}}
{{- if gt (len $subjects) 0 -}}
  {{- $fragments := list -}}
  {{- range $subjects -}}
    {{- $fragment := include "kyverno-policies.celSubjectFragment" . -}}
    {{- $fragments = append $fragments $fragment -}}
  {{- end -}}
  {{- $conditions = append $conditions (dict "name" "exclude-subjects" "expression" (printf "!(%s)" (join " || " $fragments))) -}}
{{- end -}}
{{- $passthroughConditions := .matchConditions | default list -}}
{{- range $passthroughConditions -}}
  {{- $conditions = append $conditions (dict "name" (.name | required "matchConditions entries must have a 'name'") "expression" (.expression | required "matchConditions entries must have an 'expression'")) -}}
{{- end -}}
{{- if gt (len $conditions) 0 }}
matchConditions:
{{- range $conditions }}
  - name: {{ .name }}
    expression: {{ .expression | quote }}
{{- end -}}
{{- end -}}
{{- end -}}

{{/* Generate a single CEL expression fragment for one subject entry.
     Supported kinds: User, Group, ServiceAccount.
     - User:           request.userInfo.username == '<name>'
     - Group:          '<name>' in request.userInfo.groups
     - ServiceAccount: (parseServiceAccount(request.userInfo.username).Namespace == '<ns>' && parseServiceAccount(request.userInfo.username).Name == '<name>')
*/}}
{{- define "kyverno-policies.celSubjectFragment" -}}
{{- $kind := .kind | required "excludeSubjects entries must have a 'kind' (User, Group, or ServiceAccount)" -}}
{{- $name := .name | required "excludeSubjects entries must have a 'name'" -}}
{{- if eq $kind "User" -}}
  request.userInfo.username == '{{ $name }}'
{{- else if eq $kind "Group" -}}
  '{{ $name }}' in request.userInfo.groups
{{- else if eq $kind "ServiceAccount" -}}
  {{- $ns := .namespace | required "excludeSubjects ServiceAccount entries must have a 'namespace'" -}}
  (parseServiceAccount(request.userInfo.username).Namespace == '{{ $ns }}' && parseServiceAccount(request.userInfo.username).Name == '{{ $name }}')
{{- else -}}
  {{- fail (printf "excludeSubjects: unsupported kind '%s' — must be User, Group, or ServiceAccount" $kind) -}}
{{- end -}}
{{- end -}}

{{/* Convert a list of strings to a CEL list literal: 'a', 'b', 'c' */}}
{{- define "kyverno-policies.celStringList" -}}
{{- $items := list -}}
{{- range . -}}
  {{- $items = append $items (printf "'%s'" .) -}}
{{- end -}}
{{- join ", " $items -}}
{{- end -}}
