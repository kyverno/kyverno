#!/bin/bash
echo "Kyverno Values Generator Script"

NAME_OVERRIDE_VALUE=""
FULLNAME_OVERRIDE_VALUE=""
NAMESPACE_VALUE=""
TAG_VALUE=""
CHART_LOCATION="${BASH_SOURCE%/*}/../charts/kyverno/values.yaml"

print_usage(){
    echo "
    Kyverno Values Generator Script is used in generating dynamic templates of kyverno/charts/values.yaml
    
    Usage:
    -v = Name Override Value 
    -f = Fullname Override Value
    -n = Namespace Value
    -t  = initImage tag Value
    "
}

while getopts :v:f:n:t: flag; do
    case "${flag}" in
    v | --nameOverride) NAME_OVERRIDE_VALUE=${OPTARG} ;;
    f | --fullnameOverride) FULLNAME_OVERRIDE_VALUE=${OPTARG} ;;
    n | --namespace) NAMESPACE_VALUE=${OPTARG};;
    t | --tag)  TAG_VALUE=${OPTARG};;
    esac
done

if [ -z "${NAME_OVERRIDE_VALUE}" ] || [ -z "$FULLNAME_OVERRIDE_VALUE" ] || [ -z "${NAMESPACE_VALUE}" ] || [ -z "${TAG_VALUE}" ]; then
    print_usage
    exit 1
fi

echo "
The recieved variables are:
Name Override Value: ${NAME_OVERRIDE_VALUE} 
Fullname Override Value: ${FULLNAME_OVERRIDE_VALUE}
Namespace Value: ${NAMESPACE_VALUE}
Tag Value: ${TAG_VALUE}
"
echo "Generating values.yaml"

echo "
nameOverride: ${NAME_OVERRIDE_VALUE}
fullnameOverride: ${FULLNAME_OVERRIDE_VALUE}
namespace: ${NAMESPACE_VALUE}
# -- Additional labels
customLabels: {}
rbac:
  create: true
  serviceAccount:
    create: true
    name:
    annotations: {}
    #   example.com/annotation: value
image:
  repository: ghcr.io/kyverno/kyverno
  # Defaults to appVersion in Chart.yaml if omitted
  tag: ${TAG_VALUE}
  pullPolicy: IfNotPresent
  pullSecrets: []
  # - secretName
initImage:
  repository: ghcr.io/kyverno/kyvernopre
  # If initImage.tag is missing, defaults to image.tag
  tag:  ${TAG_VALUE}
  # If initImage.pullPolicy is missing, defaults to image.pullPolicy
  pullPolicy:
  # No pull secrets just for initImage; just add to image.pullSecrets
testImage:
  # testImage.repository defaults to \"busybox\" if omitted
  repository:
  # testImage.tag defaults to \"latest\" if omitted
  tag:
  # testImage.pullPolicy defaults to image.pullPolicy if omitted
  pullPolicy:
replicaCount: 1
podLabels: {}
#   example.com/label: foo
podAnnotations: {}
#   example.com/annotation: foo
podSecurityContext: {}
# Optional priority class to be used for kyverno pods
priorityClassName: \"\"
antiAffinity:
  # This can be disabled in a 1 node cluster.
  enable: true
affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
    - weight: 1
      podAffinityTerm:
        labelSelector:
          matchExpressions:
          - key: app.kubernetes.io/name
            operator: In
            values:
            - kyverno
        topologyKey: kubernetes.io/hostname
podDisruptionBudget:
  minAvailable: 1
  # maxUnavailable: 1
  # minAvailable and maxUnavailable can either be set to an integer (e.g. 1)
  # or a percentage value (e.g. 25%)
nodeSelector: {}
tolerations: []
# change hostNetwork to true when you want the kyverno's pod to share its host's network namespace
# useful for situations like when you end up dealing with a custom CNI over Amazon EKS
# update the 'dnsPolicy' accordingly as well to suit the host network mode
hostNetwork: false
# dnsPolicy determines the manner in which DNS resolution happens in the cluster
# in case of hostNetwork: true, usually, the dnsPolicy is suitable to be \"ClusterFirstWithHostNet\"
# for further reference: https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#pod-s-dns-policy
dnsPolicy: \"ClusterFirst\"
# env variables for initContainers
envVarsInit: {}
# env variables for containers
envVars: {}
extraArgs: []
# - --webhookTimeout=4
resources:
  limits:
    memory: 384Mi
  requests:
    cpu: 100m
    memory: 128Mi
initResources:
  limits:
    cpu: 100m
    memory: 256Mi
  requests:
    cpu: 10m
    memory: 64Mi
## Liveness Probe. The block is directly forwarded into the deployment, so you can use whatever livenessProbe configuration you want.
## ref: https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-probes/
##
livenessProbe:
  httpGet:
    path: /health/liveness
    port: 9443
    scheme: HTTPS
  initialDelaySeconds: 15
  periodSeconds: 30
  timeoutSeconds: 5
  failureThreshold: 2
  successThreshold: 1
## Readiness Probe. The block is directly forwarded into the deployment, so you can use whatever readinessProbe configuration you want.
## ref: https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-probes/
##
readinessProbe:
  httpGet:
    path: /health/readiness
    port: 9443
    scheme: HTTPS
  initialDelaySeconds: 5
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 6
  successThreshold: 1
# TODO(mbarrien): Should we just list all resources for the
# generatecontroller in here rather than having defaults hard-coded?
generatecontrollerExtraResources:
# - ResourceA
# - ResourceB
config:
  # resource types to be skipped by kyverno policy engine
  # Make sure to surround each entry in quotes so that it doesn't get parsed
  # as a nested YAML list. These are joined together without spaces in the configmap
  resourceFilters:
  - \"[Event,*,*]\"
  - \"[*,kube-system,*]\"
  - \"[*,kube-public,*]\"
  - \"[*,kube-node-lease,*]\"
  - \"[Node,*,*]\"
  - \"[APIService,*,*]\"
  - \"[TokenReview,*,*]\"
  - \"[SubjectAccessReview,*,*]\"
  - \"[SelfSubjectAccessReview,*,*]\"
  - \"[*,kyverno,*]\"
  - \"[Binding,*,*]\"
  - \"[ReplicaSet,*,*]\"
  - \"[ReportChangeRequest,*,*]\"
  - \"[ClusterReportChangeRequest,*,*]\"
  # Or give the name of an existing config map (ignores default/provided resourceFilters)
  existingConfig: ''
  excludeGroupRole:
#  - \"\"
  excludeUsername:
#  - \"\"
  # Webhookconfigurations, this block defines the namespaceSelector in the webhookconfigurations.
  # Note that it takes a list of namespaceSelector in the JSON format, and only the first element
  # will be forwarded to the webhookconfigurations.
  webhooks:
  # webhooks: [{"namespaceSelector":{"matchExpressions":[{"key":"environment","operator":"In","values":["prod"]}]}}]
  generateSuccessEvents: 'false'
  # existingConfig: kyverno
  metricsConfig:
    namespaces: {
      "include": [],
      "exclude": []
    }
    # 'namespaces.include': list of namespaces to capture metrics for. Default: metrics being captured for all namespaces except excludeNamespaces.
    # 'namespaces.exclude': list of namespaces to NOT capture metrics for. Default: []
    # metricsRefreshInterval: 24h
    # rate at which metrics should reset so as to clean up the memory footprint of kyverno metrics, if you might be expecting high memory footprint of Kyverno's metrics. Default: 0, no refresh of metrics
  # Or provide an existing metrics config-map by uncommenting the below line
  # existingMetricsConfig: sample-metrics-configmap. Refer to the ./templates/metricsconfigmap.yaml for the structure of metrics configmap.
## Deployment update strategy
## Ref: https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#strategy
updateStrategy:
  rollingUpdate:
    maxSurge: 1
    maxUnavailable: 40%
  type: RollingUpdate
service:
  port: 443
  type: ClusterIP
  # Only used if service.type is NodePort
  nodePort:
  annotations: {}
topologySpreadConstraints: []
metricsService:
  create: true
  type: ClusterIP
  ## Kyverno's metrics server will be exposed at this port
  port: 8000
  ## The Node's port which will allow access Kyverno's metrics at the host level. Only used if service.type is NodePort.
  nodePort:
  ## Provide any additional annotations which may be required. This can be used to
  ## set the LoadBalancer service type to internal only.
  ## ref: https://kubernetes.io/docs/concepts/services-networking/service/#internal-load-balancer
  ##
  annotations: {}
# Service Monitor to collect Prometheus Metrics
serviceMonitor:
  enabled: false
  # Additional labels
  additionalLabels:
    # key: value
  # Override namespace (default is same than kyverno)
  namespace: ${NAMESPACE_VALUE}
  # Interval to scrape metrics
  interval: 30s
  # Timeout if metrics can't be retrieved in given time interval
  scrapeTimeout: 25s
  # Is TLS required for endpoint
  secure: false
  # TLS Configuration for endpoint
  tlsConfig: {}
# Kyverno requires a certificate key pair and corresponding certificate authority
# to properly register its webhooks. This can be done in one of 3 ways:
# 1) Use kube-controller-manager to generate a CA-signed certificate (preferred)
# 2) Provide your own CA and cert.
#    In this case, you will need to create a certificate with a specific name and data structure.
#    As long as you follow the naming scheme, it will be automatically picked up.
#    kyverno-svc.(namespace).svc.kyverno-tls-ca (with data entry named rootCA.crt)
#    kyverno-svc.kyverno.svc.kyverno-tls-pair (with data entries named tls.key and tls.crt)
# 3) Let Helm generate a self signed cert, by setting createSelfSignedCert true
# If letting Kyverno create its own CA or providing your own, make createSelfSignedCert is false
createSelfSignedCert: false
# Whether to have Helm install the Kyverno CRDs
# If the CRDs are not installed by Helm, they must be added
# before policies can be created
installCRDs: true
# When true, use a NetworkPolicy to allow ingress to the webhook
# This is useful on clusters using Calico and/or native k8s network
# policies in a default-deny setup.
networkPolicy:
  enabled: false
  namespaceExpressions: []
  namespaceLabels: {}
  podExpressions: []
  podLabels: {}
" > $CHART_LOCATION
echo "Values generated"
