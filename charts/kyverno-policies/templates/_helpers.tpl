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

{{/* Custom annotations with per-policy overrides.
     Merges default customAnnotations with per-policy overrides.
     Per-policy entries override defaults when they share the same key.
     Receives dict with "name" (policy name) and "values" (.Values). */}}
{{- define "kyverno-policies.customAnnotations" -}}
{{- $policyName := index . "name" -}}
{{- $values := index . "values" -}}
{{- $defaults := $values.customAnnotations | default dict -}}
{{- $overrides := index $values.customAnnotationsByPolicy $policyName | default dict -}}
{{- $merged := merge $overrides $defaults -}}
{{- if gt (len $merged) 0 -}}
{{- toYaml $merged -}}
{{- end -}}
{{- end -}}

{{/* Resolve severity for a specific policy.
     Per-policy entry (podSecuritySeverityByPolicy.<name>) overrides the global default (podSecuritySeverity).
     Always returns a string (handles numeric values like 1 being passed). */}}
{{- define "kyverno-policies.policySeverity" -}}
{{- $policyName := index . "name" -}}
{{- $values := index . "values" -}}
{{- $byPolicy := $values.podSecuritySeverityByPolicy | default dict -}}
{{- if hasKey $byPolicy $policyName -}}
  {{- $perPolicy := index $byPolicy $policyName -}}
  {{- if $perPolicy -}}
    {{- $perPolicy | quote -}}
  {{- end -}}
{{- else -}}
  {{- $values.podSecuritySeverity | quote -}}
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
{{- if hasKey $values.vpolExcludeByPolicy $policyName -}}
  {{- $perPolicy := index $values.vpolExcludeByPolicy $policyName | default dict -}}
  {{- if gt (len $perPolicy) 0 -}}
    {{- toYaml $perPolicy -}}
  {{- end -}}
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
  {{- $conditions = append $conditions (dict "name" "exclude-namespaces" "expression" (printf "!(request.namespace in [%s])" (include "kyverno-policies.celStringList" $namespaces))) -}}
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

{{/* Generate auditAnnotations for a specific ValidatingPolicy.
     Merges default auditAnnotations with per-policy overrides.
     Per-policy entries override defaults when they share the same key.
     Receives dict with "name" (policy name) and "values" (.Values). */}}
{{- define "kyverno-policies.policyAuditAnnotations" -}}
{{- $policyName := index . "name" -}}
{{- $values := index . "values" -}}
{{- $defaults := $values.auditAnnotations | default dict -}}
{{- $overrides := index $values.auditAnnotationsByPolicy $policyName | default dict -}}
{{- $merged := merge $defaults $overrides -}}
{{- if gt (len $merged) 0 }}
auditAnnotations:
{{- $keys := keys $merged | sortAlpha -}}
{{- range $keys }}
  - key: {{ . }}
    valueExpression: {{ index $merged . | quote }}
{{- end -}}
{{- end -}}
{{- end -}}

{{/* Oldest and newest Pod Security Admission (PSA) versions the chart implements.
     Bump psaMaxVersion when the chart is synced with a new PSA minor version. */}}
{{- define "kyverno-policies.psaMinVersion" -}}1.25{{- end -}}
{{- define "kyverno-policies.psaMaxVersion" -}}1.37{{- end -}}

{{/* "true" when podSecurityVersion is "latest". */}}
{{- define "kyverno-policies.psaLatest" -}}
{{- eq (toString (.Values.podSecurityVersion | default "latest")) "latest" -}}
{{- end -}}

{{/* PSA version to match, as "1.x".
     "latest" uses the cluster's Kubernetes minor version. Like PSA, a version newer
     than psaMaxVersion uses psaMaxVersion. A pinned version older than psaMinVersion
     or in the wrong format fails the render. */}}
{{- define "kyverno-policies.psaVersion" -}}
{{- $min := include "kyverno-policies.psaMinVersion" . -}}
{{- $max := include "kyverno-policies.psaMaxVersion" . -}}
{{- $want := toString (.Values.podSecurityVersion | default "latest") -}}
{{- $version := "" -}}
{{- if eq $want "latest" -}}
  {{- $kube := semver .Capabilities.KubeVersion.Version -}}
  {{- $version = printf "%d.%d" $kube.Major $kube.Minor -}}
{{- else if regexMatch "^v1\\.(0|[1-9][0-9]*)$" $want -}}
  {{- $version = trimPrefix "v" $want -}}
  {{- if semverCompare (printf "<%s" $min) $version -}}
    {{- fail (printf "podSecurityVersion %s is not supported, use latest or v%s to v%s" $want $min $max) -}}
  {{- end -}}
{{- else -}}
  {{- fail (printf "podSecurityVersion must be latest or v1.x, got %q. Quote the value so YAML reads it as a string, for example podSecurityVersion: \"v1.30\"" $want) -}}
{{- end -}}
{{- if semverCompare (printf ">%s" $max) $version -}}
  {{- $version = $max -}}
{{- end -}}
{{- $version -}}
{{- end -}}

{{/* Value of the kyverno.io/pod-security-version annotation.
     A pinned version shows as "v1.x". "latest" shows as "latest (v1.x)", because it keeps
     the allow-lists of earlier chart releases and can allow more than PSA v1.x. */}}
{{- define "kyverno-policies.psaVersionAnnotation" -}}
{{- if eq (include "kyverno-policies.psaLatest" .) "true" -}}
latest (v{{ include "kyverno-policies.psaVersion" . }})
{{- else -}}
v{{ include "kyverno-policies.psaVersion" . }}
{{- end -}}
{{- end -}}

{{/* "true" when the PSA version is at least .min. Receives dict with "min" ("1.x") and "ctx" (the root context). */}}
{{- define "kyverno-policies.psaAtLeast" -}}
{{- semverCompare (printf ">=%s" .min) (include "kyverno-policies.psaVersion" .ctx) -}}
{{- end -}}

{{/* "true" when the user namespace relaxation applies.
     PSA relaxes these checks from v1.35. With "latest" the podSecurityUserNamespaces
     setting alone decides, so existing installs keep their behavior. */}}
{{- define "kyverno-policies.psaUserNamespaces" -}}
{{- and .Values.podSecurityUserNamespaces (or (eq (include "kyverno-policies.psaLatest" .) "true") (eq (include "kyverno-policies.psaAtLeast" (dict "min" "1.35" "ctx" .)) "true")) -}}
{{- end -}}

{{/* CEL clause that relaxes a check for user namespace pods.
     Kubernetes Pod Security Admission relaxes only procMount (Baseline),
     runAsNonRoot and runAsUser (Restricted) when spec.hostUsers is explicitly
     false. An absent or true hostUsers must never relax, so the clause tests
     for an explicit false. Emits nothing unless the relaxation applies (see
     psaUserNamespaces), so call sites guard it with `with` and indent it themselves.
     Do not use it in any other policy. */}}
{{- define "kyverno-policies.userNamespaceRelaxation" -}}
{{- if eq (include "kyverno-policies.psaUserNamespaces" .) "true" -}}
{{- "object.spec.?hostUsers.orValue(true) == false ||" -}}
{{- end -}}
{{- end -}}
<<<<<<< HEAD
=======

{{/* Safe sysctls allowed by PSA for the PSA version.
     With "latest" the list never drops below the v1.29 one, so Pods that pass today keep passing. */}}
{{- define "kyverno-policies.allowedSysctls" -}}
{{- $latest := eq (include "kyverno-policies.psaLatest" .) "true" -}}
{{- $sysctls := list "kernel.shm_rmid_forced" "net.ipv4.ip_local_port_range" "net.ipv4.tcp_syncookies" "net.ipv4.ping_group_range" "net.ipv4.ip_unprivileged_port_start" -}}
{{- if or $latest (eq (include "kyverno-policies.psaAtLeast" (dict "min" "1.27" "ctx" .)) "true") -}}
{{- $sysctls = append $sysctls "net.ipv4.ip_local_reserved_ports" -}}
{{- end -}}
{{- if or $latest (eq (include "kyverno-policies.psaAtLeast" (dict "min" "1.29" "ctx" .)) "true") -}}
{{- $sysctls = concat $sysctls (list "net.ipv4.tcp_keepalive_time" "net.ipv4.tcp_fin_timeout" "net.ipv4.tcp_keepalive_intvl" "net.ipv4.tcp_keepalive_probes") -}}
{{- end -}}
{{- if eq (include "kyverno-policies.psaAtLeast" (dict "min" "1.32" "ctx" .)) "true" -}}
{{- $sysctls = concat $sysctls (list "net.ipv4.tcp_rmem" "net.ipv4.tcp_wmem") -}}
{{- end -}}
{{- if eq (include "kyverno-policies.psaAtLeast" (dict "min" "1.37" "ctx" .)) "true" -}}
{{- $sysctls = concat $sysctls (list "net.ipv4.tcp_slow_start_after_idle" "net.ipv4.tcp_notsent_lowat") -}}
{{- end -}}
{{- range $i, $s := $sysctls }}{{ if $i }}, {{ end }}'{{ $s }}'{{ end -}}
{{- end -}}

{{/* SELinux types allowed by PSA for the PSA version, as a CEL list body.
     container_engine_t is allowed from v1.31, and always with "latest" to keep today's behavior. */}}
{{- define "kyverno-policies.allowedSELinuxTypes" -}}
{{- $types := list "" "container_t" "container_init_t" "container_kvm_t" -}}
{{- if or (eq (include "kyverno-policies.psaLatest" .) "true") (eq (include "kyverno-policies.psaAtLeast" (dict "min" "1.31" "ctx" .)) "true") -}}
{{- $types = append $types "container_engine_t" -}}
{{- end -}}
{{- include "kyverno-policies.celStringList" $types -}}
{{- end -}}
>>>>>>> c07352676 (feat(charts): select the Pod Security Admission version for policies (#18268))
