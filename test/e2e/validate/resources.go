package validate

import "fmt"

// Namespace Description
var namespaceYaml = []byte(`
apiVersion: v1
kind: Namespace
metadata:
  name: test-validate
`)

func newNamespaceYaml(name string) []byte {
	ns := fmt.Sprintf(`
  apiVersion: v1
  kind: Namespace
  metadata:
    name: %s
  `, name)
	return []byte(ns)
}

// Regression: https://github.com/kyverno/kyverno/issues/2043
// Policy: https://github.com/fluxcd/flux2-multi-tenancy/blob/main/infrastructure/kyverno-policies/flux-multi-tenancy.yaml
var kyverno_2043_policy = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: flux-multi-tenancy
spec:
  validationFailureAction: enforce
  rules:
    - name: serviceAccountName
      exclude:
        resources:
          namespaces:
            - flux-system
      match:
        resources:
          kinds:
            - Kustomization
            - HelmRelease
      validate:
        message: ".spec.serviceAccountName is required"
        pattern:
          spec:
            serviceAccountName: "?*"
    - name: sourceRefNamespace
      exclude:
        resources:
          namespaces:
            - flux-system
      match:
        resources:
          kinds:
            - Kustomization
            - HelmRelease
      validate:
        message: "spec.sourceRef.namespace must be the same as metadata.namespace"
        deny:
          conditions:
            - key: "{{request.object.spec.sourceRef.namespace}}"
              operator: NotEquals
              value:  "{{request.object.metadata.namespace}}"
`)

var kyverno_2241_policy = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: flux-multi-tenancy
spec:
  validationFailureAction: enforce
  rules:
    - name: serviceAccountName
      exclude:
        resources:
          namespaces:
            - flux-system
      match:
        resources:
          kinds:
            - Kustomization
            - HelmRelease
      validate:
        message: ".spec.serviceAccountName is required"
        pattern:
          spec:
            serviceAccountName: "?*"
    - name: sourceRefNamespace
      exclude:
        resources:
          namespaces:
            - flux-system
      match:
        resources:
          kinds:
            - Kustomization
            - HelmRelease
      preconditions:
        any:
        - key: "{{request.object.spec.sourceRef.namespace}}"
          operator: NotEquals
          value: ""
      validate:
        message: "spec.sourceRef.namespace must be the same as metadata.namespace"
        deny:
          conditions:
            - key: "{{request.object.spec.sourceRef.namespace}}"
              operator: NotEquals
              value:  "{{request.object.metadata.namespace}}"
`)

var kyverno_2043_FluxCRD = []byte(`
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.5.0
  creationTimestamp: null
  name: kustomizations.kustomize.toolkit.fluxcd.io
spec:
  group: kustomize.toolkit.fluxcd.io
  names:
    kind: Kustomization
    listKind: KustomizationList
    plural: kustomizations
    shortNames:
    - ks
    singular: kustomization
  scope: Namespaced
  versions:
  - additionalPrinterColumns:
    - jsonPath: .status.conditions[?(@.type=="Ready")].status
      name: Ready
      type: string
    - jsonPath: .status.conditions[?(@.type=="Ready")].message
      name: Status
      type: string
    - jsonPath: .metadata.creationTimestamp
      name: Age
      type: date
    name: v1beta1
    schema:
      openAPIV3Schema:
        description: Kustomization is the Schema for the kustomizations API.
        properties:
          apiVersion:
            description: 'APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources'
            type: string
          kind:
            description: 'Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
            type: string
          metadata:
            type: object
          spec:
            description: KustomizationSpec defines the desired state of a kustomization.
            properties:
              decryption:
                description: Decrypt Kubernetes secrets before applying them on the cluster.
                properties:
                  provider:
                    description: Provider is the name of the decryption engine.
                    enum:
                    - sops
                    type: string
                  secretRef:
                    description: The secret name containing the private OpenPGP keys used for decryption.
                    properties:
                      name:
                        description: Name of the referent
                        type: string
                    required:
                    - name
                    type: object
                required:
                - provider
                type: object
              dependsOn:
                description: DependsOn may contain a dependency.CrossNamespaceDependencyReference slice with references to Kustomization resources that must be ready before this Kustomization can be reconciled.
                items:
                  description: CrossNamespaceDependencyReference holds the reference to a dependency.
                  properties:
                    name:
                      description: Name holds the name reference of a dependency.
                      type: string
                    namespace:
                      description: Namespace holds the namespace reference of a dependency.
                      type: string
                  required:
                  - name
                  type: object
                type: array
              force:
                default: false
                description: Force instructs the controller to recreate resources when patching fails due to an immutable field change.
                type: boolean
              healthChecks:
                description: A list of resources to be included in the health assessment.
                items:
                  description: NamespacedObjectKindReference contains enough information to let you locate the typed referenced object in any namespace
                  properties:
                    apiVersion:
                      description: API version of the referent, if not specified the Kubernetes preferred version will be used
                      type: string
                    kind:
                      description: Kind of the referent
                      type: string
                    name:
                      description: Name of the referent
                      type: string
                    namespace:
                      description: Namespace of the referent, when not specified it acts as LocalObjectReference
                      type: string
                  required:
                  - kind
                  - name
                  type: object
                type: array
              images:
                description: Images is a list of (image name, new name, new tag or digest) for changing image names, tags or digests. This can also be achieved with a patch, but this operator is simpler to specify.
                items:
                  description: Image contains an image name, a new name, a new tag or digest, which will replace the original name and tag.
                  properties:
                    digest:
                      description: Digest is the value used to replace the original image tag. If digest is present NewTag value is ignored.
                      type: string
                    name:
                      description: Name is a tag-less image name.
                      type: string
                    newName:
                      description: NewName is the value used to replace the original name.
                      type: string
                    newTag:
                      description: NewTag is the value used to replace the original tag.
                      type: string
                  required:
                  - name
                  type: object
                type: array
              interval:
                description: The interval at which to reconcile the Kustomization.
                type: string
              kubeConfig:
                description: The KubeConfig for reconciling the Kustomization on a remote cluster. When specified, KubeConfig takes precedence over ServiceAccountName.
                properties:
                  secretRef:
                    description: SecretRef holds the name to a secret that contains a 'value' key with the kubeconfig file as the value. It must be in the same namespace as the Kustomization. It is recommended that the kubeconfig is self-contained, and the secret is regularly updated if credentials such as a cloud-access-token expire.
                    properties:
                      name:
                        description: Name of the referent
                        type: string
                    required:
                    - name
                    type: object
                type: object
              patches:
                description: Patches (also called overlays), defined as inline YAML objects.
                items:
                  description: Patch contains either a StrategicMerge or a JSON6902 patch, either a file or inline, and the target the patch should be applied to.
                  properties:
                    patch:
                      description: Patch contains the JSON6902 patch document with an array of operation objects.
                      type: string
                    target:
                      description: Target points to the resources that the patch document should be applied to.
                      properties:
                        annotationSelector:
                          description: AnnotationSelector is a string that follows the label selection expression https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#api It matches with the resource annotations.
                          type: string
                        group:
                          description: Group is the API group to select resources from. Together with Version and Kind it is capable of unambiguously identifying and/or selecting resources. https://github.com/kubernetes/community/blob/master/contributors/design-proposals/api-machinery/api-group.md
                          type: string
                        kind:
                          description: Kind of the API Group to select resources from. Together with Group and Version it is capable of unambiguously identifying and/or selecting resources. https://github.com/kubernetes/community/blob/master/contributors/design-proposals/api-machinery/api-group.md
                          type: string
                        labelSelector:
                          description: LabelSelector is a string that follows the label selection expression https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#api It matches with the resource labels.
                          type: string
                        name:
                          description: Name to match resources with.
                          type: string
                        namespace:
                          description: Namespace to select resources from.
                          type: string
                        version:
                          description: Version of the API Group to select resources from. Together with Group and Kind it is capable of unambiguously identifying and/or selecting resources. https://github.com/kubernetes/community/blob/master/contributors/design-proposals/api-machinery/api-group.md
                          type: string
                      type: object
                  type: object
                type: array
              patchesJson6902:
                description: JSON 6902 patches, defined as inline YAML objects.
                items:
                  description: JSON6902Patch contains a JSON6902 patch and the target the patch should be applied to.
                  properties:
                    patch:
                      description: Patch contains the JSON6902 patch document with an array of operation objects.
                      items:
                        description: JSON6902 is a JSON6902 operation object. https://tools.ietf.org/html/rfc6902#section-4
                        properties:
                          from:
                            type: string
                          op:
                            enum:
                            - test
                            - remove
                            - add
                            - replace
                            - move
                            - copy
                            type: string
                          path:
                            type: string
                          value:
                            x-kubernetes-preserve-unknown-fields: true
                        required:
                        - op
                        - path
                        type: object
                      type: array
                    target:
                      description: Target points to the resources that the patch document should be applied to.
                      properties:
                        annotationSelector:
                          description: AnnotationSelector is a string that follows the label selection expression https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#api It matches with the resource annotations.
                          type: string
                        group:
                          description: Group is the API group to select resources from. Together with Version and Kind it is capable of unambiguously identifying and/or selecting resources. https://github.com/kubernetes/community/blob/master/contributors/design-proposals/api-machinery/api-group.md
                          type: string
                        kind:
                          description: Kind of the API Group to select resources from. Together with Group and Version it is capable of unambiguously identifying and/or selecting resources. https://github.com/kubernetes/community/blob/master/contributors/design-proposals/api-machinery/api-group.md
                          type: string
                        labelSelector:
                          description: LabelSelector is a string that follows the label selection expression https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#api It matches with the resource labels.
                          type: string
                        name:
                          description: Name to match resources with.
                          type: string
                        namespace:
                          description: Namespace to select resources from.
                          type: string
                        version:
                          description: Version of the API Group to select resources from. Together with Group and Kind it is capable of unambiguously identifying and/or selecting resources. https://github.com/kubernetes/community/blob/master/contributors/design-proposals/api-machinery/api-group.md
                          type: string
                      type: object
                  required:
                  - patch
                  - target
                  type: object
                type: array
              patchesStrategicMerge:
                description: Strategic merge patches, defined as inline YAML objects.
                items:
                  x-kubernetes-preserve-unknown-fields: true
                type: array
              path:
                description: Path to the directory containing the kustomization.yaml file, or the set of plain YAMLs a kustomization.yaml should be generated for. Defaults to 'None', which translates to the root path of the SourceRef.
                type: string
              postBuild:
                description: PostBuild describes which actions to perform on the YAML manifest generated by building the kustomize overlay.
                properties:
                  substitute:
                    additionalProperties:
                      type: string
                    description: Substitute holds a map of key/value pairs. The variables defined in your YAML manifests that match any of the keys defined in the map will be substituted with the set value. Includes support for bash string replacement functions e.g. ${var:=default}, ${var:position} and ${var/substring/replacement}.
                    type: object
                  substituteFrom:
                    description: SubstituteFrom holds references to ConfigMaps and Secrets containing the variables and their values to be substituted in the YAML manifests. The ConfigMap and the Secret data keys represent the var names and they must match the vars declared in the manifests for the substitution to happen.
                    items:
                      description: SubstituteReference contains a reference to a resource containing the variables name and value.
                      properties:
                        kind:
                          description: Kind of the values referent, valid values are ('Secret', 'ConfigMap').
                          enum:
                          - Secret
                          - ConfigMap
                          type: string
                        name:
                          description: Name of the values referent. Should reside in the same namespace as the referring resource.
                          maxLength: 253
                          minLength: 1
                          type: string
                      required:
                      - kind
                      - name
                      type: object
                    type: array
                type: object
              prune:
                description: Prune enables garbage collection.
                type: boolean
              retryInterval:
                description: The interval at which to retry a previously failed reconciliation. When not specified, the controller uses the KustomizationSpec.Interval value to retry failures.
                type: string
              serviceAccountName:
                description: The name of the Kubernetes service account to impersonate when reconciling this Kustomization.
                type: string
              sourceRef:
                description: Reference of the source where the kustomization file is.
                properties:
                  apiVersion:
                    description: API version of the referent
                    type: string
                  kind:
                    description: Kind of the referent
                    enum:
                    - GitRepository
                    - Bucket
                    type: string
                  name:
                    description: Name of the referent
                    type: string
                  namespace:
                    description: Namespace of the referent, defaults to the Kustomization namespace
                    type: string
                required:
                - kind
                - name
                type: object
              suspend:
                description: This flag tells the controller to suspend subsequent kustomize executions, it does not apply to already started executions. Defaults to false.
                type: boolean
              targetNamespace:
                description: TargetNamespace sets or overrides the namespace in the kustomization.yaml file.
                maxLength: 63
                minLength: 1
                type: string
              timeout:
                description: Timeout for validation, apply and health checking operations. Defaults to 'Interval' duration.
                type: string
              validation:
                description: Validate the Kubernetes objects before applying them on the cluster. The validation strategy can be 'client' (local dry-run), 'server' (APIServer dry-run) or 'none'. When 'Force' is 'true', validation will fallback to 'client' if set to 'server' because server-side validation is not supported in this scenario.
                enum:
                - none
                - client
                - server
                type: string
            required:
            - interval
            - prune
            - sourceRef
            type: object
          status:
            description: KustomizationStatus defines the observed state of a kustomization.
            properties:
              conditions:
                items:
                  description: "Condition contains details for one aspect of the current state of this API Resource. --- This struct is intended for direct use as an array at the field path .status.conditions."
                  properties:
                    lastTransitionTime:
                      description: lastTransitionTime is the last time the condition transitioned from one status to another. This should be when the underlying condition changed.  If that is not known, then using the time when the API field changed is acceptable.
                      format: date-time
                      type: string
                    message:
                      description: message is a human readable message indicating details about the transition. This may be an empty string.
                      maxLength: 32768
                      type: string
                    observedGeneration:
                      description: observedGeneration represents the .metadata.generation that the condition was set based upon. For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date with respect to the current state of the instance.
                      format: int64
                      minimum: 0
                      type: integer
                    reason:
                      description: reason contains a programmatic identifier indicating the reason for the condition's last transition. Producers of specific condition types may define expected values and meanings for this field, and whether the values are considered a guaranteed API. The value should be a CamelCase string. This field may not be empty.
                      maxLength: 1024
                      minLength: 1
                      pattern: ^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$
                      type: string
                    status:
                      description: status of the condition, one of True, False, Unknown.
                      enum:
                      - "True"
                      - "False"
                      - Unknown
                      type: string
                    type:
                      description: type of condition in CamelCase or in foo.example.com/CamelCase. --- Many .condition.type values are consistent across resources like Available, but because arbitrary conditions can be useful (see .node.status.conditions), the ability to deconflict is important. The regex it matches is (dns1123SubdomainFmt/)?(qualifiedNameFmt)
                      maxLength: 316
                      pattern: ^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*/)?(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])$
                      type: string
                  required:
                  - lastTransitionTime
                  - message
                  - reason
                  - status
                  - type
                  type: object
                type: array
              lastAppliedRevision:
                description: The last successfully applied revision. The revision format for Git sources is <branch|tag>/<commit-sha>.
                type: string
              lastAttemptedRevision:
                description: LastAttemptedRevision is the revision of the last reconciliation attempt.
                type: string
              lastHandledReconcileAt:
                description: LastHandledReconcileAt holds the value of the most recent reconcile request value, so a change can be detected.
                type: string
              observedGeneration:
                description: ObservedGeneration is the last reconciled generation.
                format: int64
                type: integer
              snapshot:
                description: The last successfully applied revision metadata.
                properties:
                  checksum:
                    description: The manifests sha1 checksum.
                    type: string
                  entries:
                    description: A list of Kubernetes kinds grouped by namespace.
                    items:
                      description: Snapshot holds the metadata of namespaced Kubernetes objects
                      properties:
                        kinds:
                          additionalProperties:
                            type: string
                          description: The list of Kubernetes kinds.
                          type: object
                        namespace:
                          description: The namespace of this entry.
                          type: string
                      required:
                      - kinds
                      type: object
                    type: array
                required:
                - checksum
                - entries
                type: object
            type: object
        type: object
    served: true
    storage: true
    subresources:
      status: {}
status:
  acceptedNames:
    kind: ""
    plural: ""
  conditions: []
  storedVersions: []
`)

var kyverno_2043_FluxKustomization = []byte(`
apiVersion: kustomize.toolkit.fluxcd.io/v1beta1
kind: Kustomization
metadata:
  name: dev-team
  namespace: test-validate
spec:
  serviceAccountName: dev-team
  interval: 5m
  sourceRef:
    kind: GitRepository
    name: dev-team
  prune: true
  validation: client
`)

var kyverno_2241_FluxKustomization = []byte(`
apiVersion: kustomize.toolkit.fluxcd.io/v1beta1
kind: Kustomization
metadata:
  name: tenants
  namespace: test-validate
spec:
  serviceAccountName: dev-team
  interval: 5m
  sourceRef:
    kind: GitRepository
    name: flux-system
  path: ./tenants/production
  prune: true
  validation: client
`)

var kyverno_2345_policy = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: drop-cap-net-raw
spec:
  validationFailureAction: enforce
  background: false
  rules:
  - name: drop-cap-net-raw
    match:
      resources:
        kinds:
        - Pod
    validate:
      deny:
        conditions:
          any:
          - key: "{{ request.object.spec.containers[].securityContext.capabilities.drop[] | contains(@, 'NET_RAW') }}"
            operator: Equals
            value: false
`)

var kyverno_2345_resource = []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: test
  namespace: test-validate1
spec:
  initContainers:
  - name: jimmy
    image: defdasdabian:923
    command: ["/bin/sh", "-c", "sleep infinity"]
    securityContext:
      capabilities:
        drop:
        - XXXNET_RAWYYY
        - SETUID
  containers:
  - name: test
    image: defdasdabian:923
    command: ["/bin/sh", "-c", "sleep infinity"]
    securityContext:
      capabilities:
        drop:
        - XXXNET_RAWYYY
        - SETUID
        - CAP_FOO_BAR
  - name: asdf
    image: defdasdabian:923
    command: ["/bin/sh", "-c", "sleep infinity"]
    securityContext:
      capabilities:
        drop:
        - CAP_SOMETHING
`)

var kyverno_trustable_image_policy = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: check-trustable-images
spec:
  validationFailureAction: enforce
  rules:
  - name: only-allow-trusted-images
    match:
      resources:
        kinds:
        - Pod
    preconditions:
      - key: "{{request.operation}}"
        operator: NotEquals
        value: DELETE
    validate:
      message: "images with root user are not allowed"
      foreach:
      - list: "request.object.spec.containers"
        context:
        - name: imageData
          imageRegistry:
            reference: "{{ element.image }}"
            jmesPath: "{user: configData.config.User || '', registry: registry}"
        deny:
          conditions:
            all:
              - key: "{{ imageData.user }}"
                operator: Equals
                value: ""
              - key: "{{ imageData.registry }}"
                operator: NotEquals
                value: "ghcr.io"
`)

var kyverno_global_anchor_validate_policy = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: sample
spec:
  validationFailureAction: enforce
  rules:
  - name: check-container-image
    match:
      resources:
        kinds:
        - Pod
    validate:
      pattern:
        spec:
          containers:
          - name: "*"
            <(image): "nginx"
          imagePullSecrets:
          - name: my-registry-secret
`)

var kyverno_global_anchor_validate_resource_1 = []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: pod-with-nginx-allowed-registory
spec:
  containers:
  - name: nginx
    image: nginx
  imagePullSecrets:
  - name: my-registry-secret
`)

var kyverno_global_anchor_validate_resource_2 = []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: pod-with-nginx-disallowed-registory
spec:
  containers:
  - name: nginx
    image: nginx
  imagePullSecrets:
  - name: other-registory-secret
`)

var kyverno_trusted_image_pod = []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: pod-with-trusted-registry
spec:
  containers:
  - name: kyverno
    image: ghcr.io/kyverno/kyverno:latest
`)

var kyverno_pod_with_root_user = []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: pod-with-root-user
spec:
  containers:
  - name: ubuntu
    image: ubuntu:bionic
`)

var kyverno_small_image_policy = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: images
spec:
  validationFailureAction: enforce
  rules:
  - name: only-allow-small-images
    match:
      resources:
        kinds:
        - Pod
    preconditions:
      - key: "{{request.operation}}"
        operator: NotEquals
        value: DELETE
    validate:
      message: "images with size greater than 2Gi not allowed"
      foreach:
      - list: "request.object.spec.containers"
        context:
        - name: imageSize
          imageRegistry:
            reference: "{{ element.image }}"
            # Note that we need to use "to_string" here to allow kyverno to treat it like a resource quantity of type memory
            # the total size of an image as calculated by docker is the total sum of its layer sizes
            jmesPath: "to_string(sum(manifest.layers[*].size))"
        deny:
          conditions:
            - key: "2Gi"
              operator: LessThan
              value: "{{imageSize}}"
`)

var kyverno_pod_with_small_image = []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: small-image
spec:
  containers:
  - name: small-image
    image: busybox:latest
`)

var kyverno_pod_with_large_image = []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: large-image
spec:
  containers:
  - name: large-image
    image: nvidia/cuda:11.6.0-devel-ubi8
`)
