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

var kyverno_yaml_signing_validate_policy = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: validate-resources
spec:
  validationFailureAction: enforce
  background: false
  webhookTimeoutSeconds: 30
  failurePolicy: Fail  
  rules:
    - name: validate-resources
      match:
        any:
        - resources:
            kinds:
              - Deployment
              - Pod
            name: test*
      exclude:
        any:
        - resources:
            kinds:
              - Pod
          subjects:
          - kind: ServiceAccount
            namespace: kube-system
            name: replicaset-controller
        - resources:
            kinds:
              - ReplicaSet
          subjects:
          - kind: ServiceAccount
            namespace: kube-system
            name: deployment-controller
      validate:
        manifests:
          attestors:
          - entries:
            - keys: 
                publicKeys:  |-
                  -----BEGIN PUBLIC KEY-----
                  MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEyQfmL5YwHbn9xrrgG3vgbU0KJxMY
                  BibYLJ5L4VSMvGxeMLnBGdM48w5IE//6idUPj3rscigFdHs7GDMH4LLAng==
                  -----END PUBLIC KEY-----
`)

var kyverno_yaml_signing_validate_resource_1 = []byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: nginx
  name: test-deployment
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
        - image: nginx:1.14.2
          name: nginx
          ports:
            - containerPort: 80

`)

var kyverno_yaml_signing_validate_resource_2 = []byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    cosign.sigstore.dev/message: H4sIAAAAAAAA/wBaAaX+H4sIAAAAAAAA/+ySz27bMAzGffZT8AUcSf6TpDrvuMMOw64DazOeEP2bxBZtn35wnXhegOW4oYB/F9rg930gQYlnTOIU7EApC/8mlDye7c9xqNk/Stc49902rn1ppZRy9OKr6IOLiXI2fqwYUzW+KXmQDw9tUx8FU+ZqoGjDqyPPu1d0tigm775t3+th371XWc//E12zL1Rbq042XacOhWzquusKkMU/4CkzpkLKdH4awh1dZjyd7vQvuyz1g4DRfKOUTfAaMMYsnlV5Nn7Q8Gk5Y+mIcUBGXQJYfCSbpy+YDBr8aPxLCeDRkYabF1DmSP0kThSt6TFrUCVAJks9hzTHOOT+x+dV7k0yk4sWmS7q1TAT9g/jjRXgOsBEHzyj8ZRW8gqMw5EuFq12qt3VS/e61u+8mRgSr0LmoCX+S0is4SjL/33djY2Njb/zKwAA//+MAMwjAAgAAAEAAP//7NcJ9loBAAA=
    cosign.sigstore.dev/signature: MEUCICLCfb3LGKXcdKV3gTXl6qba3T2goZMbVX/54gyNR05UAiEAlvPuWVsCPuBx5wVqvtyT7hr/AfR9Fl7cNLDACaNIbx8=
  labels:
    app: nginx
  name: test-deployment
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - image: nginx:1.14.2
        name: nginx
        ports:
        - containerPort: 80
`)

var kyverno_decode_x509_certificate_policy = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: test-x509-decode
spec:
  validationFailureAction: enforce
  rules:
  - name: test-x509-decode
    match:
      any:
      - resources:
          kinds:
          - ConfigMap
    validate:
      message: "public key modulus mismatch: \"{{ x509_decode('{{request.object.data.cert}}').PublicKey.N }}\" != \"{{ x509_decode('{{base64_decode('{{request.object.data.certB64}}')}}').PublicKey.N }}\""
      deny:
        conditions:
          any:
            - key: "{{ x509_decode('{{request.object.data.cert}}').PublicKey.N }}"
              operator: NotEquals
              value: "{{ x509_decode('{{base64_decode('{{request.object.data.certB64}}')}}').PublicKey.N }}"
`)

var kyverno_decode_x509_certificate_resource_fail = []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-configmap
  namespace: test-validate
data:
  cert: |
    -----BEGIN CERTIFICATE-----
    MIIDSjCCAjKgAwIBAgIUWxmj40l+TDVJq98Xy7c6Leo3np8wDQYJKoZIhvcNAQEL
    BQAwPTELMAkGA1UEBhMCeHgxCjAIBgNVBAgTAXgxCjAIBgNVBAcTAXgxCjAIBgNV
    BAoTAXgxCjAIBgNVBAsTAXgwHhcNMTgwMjAyMTIzODAwWhcNMjMwMjAxMTIzODAw
    WjA9MQswCQYDVQQGEwJ4eDEKMAgGA1UECBMBeDEKMAgGA1UEBxMBeDEKMAgGA1UE
    ChMBeDEKMAgGA1UECxMBeDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEB
    ANHkqOmVf23KMXdaZU2eFUx1h4wb09JINBB8x/HL7UE0KFJcnOoVnNQB0gRukUop
    iYCzrzMFyGWWmB/pAEKool+ZiI2uMy6mcYBDtOi4pOm7U0TQQMV6L/5Yfi65xRz3
    RTMd/tYAoFi4aCZbJAGjxU6UWNYDzTy8E/cP6ZnlNbVHRiA6/wHsoWcXtWTXYP5y
    n9cf7EWQi1hOBM4BWmOIyB1f6LEgQipZWMOMPPHO3hsuSBn0rk7jovSt5XTlbgRr
    txqAJiNjJUykWzIF+lLnZCioippGv5vkdGvE83JoACXvZTUwzA+MLu49fkw3bweq
    kbhrer8kacjfGlw3aJN37eECAwEAAaNCMEAwDgYDVR0PAQH/BAQDAgEGMA8GA1Ud
    EwEB/wQFMAMBAf8wHQYDVR0OBBYEFKXcb52bv6oqnD+D9fTNFHZL8IWxMA0GCSqG
    SIb3DQEBCwUAA4IBAQADvKvv3ym0XAYwKxPLLl3Lc6sJYHDbTN0donduG7PXeb1d
    huukJ2lfufUYp2IGSAxuLecTYeeByOVp1gaMb5LsIGt2BVDmlMMkiH29LUHsvbyi
    85CpJo7A5RJG6AWW2VBCiDjz5v8JFM6pMkBRFfXH+pwIge65CE+MTSQcfb1/aIIo
    Q226P7E/3uUGX4k4pDXG/O7GNvykF40v1DB5y7DDBTQ4JWiJfyGkT69TmdOGLFAm
    jwxUjWyvEey4qJex/EGEm5RQcMv9iy7tba1wK7sykNGn5uDELGPGIIEAa5rIHm1F
    UFOZZVoELaasWS559wy8og39Eq21dDMynb8Bndn/
    -----END CERTIFICATE-----
  certB64: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUM3VENDQWRXZ0F3SUJBZ0lCQURBTkJna3Foa2lHOXcwQkFRc0ZBREFZTVJZd0ZBWURWUVFEREEwcUxtdDUKZG1WeWJtOHVjM1pqTUI0WERUSXlNREV4TVRFek1qWTBNMW9YRFRJek1ERXhNVEUwTWpZME0xb3dHREVXTUJRRwpBMVVFQXd3TktpNXJlWFpsY201dkxuTjJZekNDQVNJd0RRWUpLb1pJaHZjTkFRRUJCUUFEZ2dFUEFEQ0NBUW9DCmdnRUJBTXNBejg1K3lpbm8rTW1kS3NWdEh3Tmkzb0FWanVtelhIaUxmVUpLN3hpNUtVOEI3Z29QSEYvVkNlL1YKN1kyYzRhZnlmZ1kyZVB3NEx4U0RrQ1lOZ1l3cWpTd0dJYmNzcXY1WlJhekJkRHhSMDlyaTZQa25OeUJWR0xpNQpSbFBYSXJHUTNwc051ZjU1cXd4SnhMTzMxcUNadXZrdEtZNVl2dUlSNEpQbUJodVNGWE9ubjBaaVF3OHV4TWNRCjBRQTJseitQeFdDVk5rOXErMzFINURIMW9ZWkRMZlUzbWlqSU9BK0FKR1piQmIrWndCbXBWTDArMlRYTHhFNzQKV293ZEtFVitXVHNLb2pOVGQwVndjdVJLUktSLzZ5blhBQWlzMjF5MVg3VWk5RkpFNm1ESXlsVUQ0MFdYT0tHSgoxbFlZNDFrUm5ZaFZodlhZTjlKdE5ZZFkzSHNDQXdFQUFhTkNNRUF3RGdZRFZSMFBBUUgvQkFRREFnS2tNQThHCkExVWRFd0VCL3dRRk1BTUJBZjh3SFFZRFZSME9CQllFRk9ubEFTVkQ5ZnUzVEFqcHRsVy9nQVhBNHFsK01BMEcKQ1NxR1NJYjNEUUVCQ3dVQUE0SUJBUUNJcHlSaUNoeHA5N2NyS2ZRMjRKdDd6OFArQUdwTGYzc1g0ZUw4N0VTYQo3UVJvVkp0WExtYXV0MXBVRW9ZTFFydUttaC8wWUZ0Wkc5V3hWZ1k2aXVLYldudTdiT2VNQi9JcitWL3lyWDNSCitYdlpPc3VYaUpuRWJKaUJXNmxKekxsZG9XNGYvNzFIK2oxV0Q0dEhwcW1kTXhxL3NMcVhmUEl1YzAvbTB5RkMKbitBREJXR0dCOE5uNjZ2eHR2K2NUNnArUklWb3RYUFFXYk1pbFdwNnBkNXdTdUI2OEZxckR3dFlMTkp0UHdGcwo5TVBWa3VhSmRZWjBlV2Qvck1jS0Q5NEhnZjg5Z3ZBMCtxek1WRmYrM0JlbVhza2pRUll5NkNLc3FveUM2alg0Cm5oWWp1bUFQLzdwc2J6SVRzbnBIdGZDRUVVKzJKWndnTTQwNmFpTWNzZ0xiCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
`)

var kyverno_decode_x509_certificate_resource_pass = []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-configmap
  namespace: test-validate
data:
  cert: |
    -----BEGIN CERTIFICATE-----
    MIIDSjCCAjKgAwIBAgIUWxmj40l+TDVJq98Xy7c6Leo3np8wDQYJKoZIhvcNAQEL
    BQAwPTELMAkGA1UEBhMCeHgxCjAIBgNVBAgTAXgxCjAIBgNVBAcTAXgxCjAIBgNV
    BAoTAXgxCjAIBgNVBAsTAXgwHhcNMTgwMjAyMTIzODAwWhcNMjMwMjAxMTIzODAw
    WjA9MQswCQYDVQQGEwJ4eDEKMAgGA1UECBMBeDEKMAgGA1UEBxMBeDEKMAgGA1UE
    ChMBeDEKMAgGA1UECxMBeDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEB
    ANHkqOmVf23KMXdaZU2eFUx1h4wb09JINBB8x/HL7UE0KFJcnOoVnNQB0gRukUop
    iYCzrzMFyGWWmB/pAEKool+ZiI2uMy6mcYBDtOi4pOm7U0TQQMV6L/5Yfi65xRz3
    RTMd/tYAoFi4aCZbJAGjxU6UWNYDzTy8E/cP6ZnlNbVHRiA6/wHsoWcXtWTXYP5y
    n9cf7EWQi1hOBM4BWmOIyB1f6LEgQipZWMOMPPHO3hsuSBn0rk7jovSt5XTlbgRr
    txqAJiNjJUykWzIF+lLnZCioippGv5vkdGvE83JoACXvZTUwzA+MLu49fkw3bweq
    kbhrer8kacjfGlw3aJN37eECAwEAAaNCMEAwDgYDVR0PAQH/BAQDAgEGMA8GA1Ud
    EwEB/wQFMAMBAf8wHQYDVR0OBBYEFKXcb52bv6oqnD+D9fTNFHZL8IWxMA0GCSqG
    SIb3DQEBCwUAA4IBAQADvKvv3ym0XAYwKxPLLl3Lc6sJYHDbTN0donduG7PXeb1d
    huukJ2lfufUYp2IGSAxuLecTYeeByOVp1gaMb5LsIGt2BVDmlMMkiH29LUHsvbyi
    85CpJo7A5RJG6AWW2VBCiDjz5v8JFM6pMkBRFfXH+pwIge65CE+MTSQcfb1/aIIo
    Q226P7E/3uUGX4k4pDXG/O7GNvykF40v1DB5y7DDBTQ4JWiJfyGkT69TmdOGLFAm
    jwxUjWyvEey4qJex/EGEm5RQcMv9iy7tba1wK7sykNGn5uDELGPGIIEAa5rIHm1F
    UFOZZVoELaasWS559wy8og39Eq21dDMynb8Bndn/
    -----END CERTIFICATE-----
  certB64: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURTakNDQWpLZ0F3SUJBZ0lVV3htajQwbCtURFZKcTk4WHk3YzZMZW8zbnA4d0RRWUpLb1pJaHZjTkFRRUwKQlFBd1BURUxNQWtHQTFVRUJoTUNlSGd4Q2pBSUJnTlZCQWdUQVhneENqQUlCZ05WQkFjVEFYZ3hDakFJQmdOVgpCQW9UQVhneENqQUlCZ05WQkFzVEFYZ3dIaGNOTVRnd01qQXlNVEl6T0RBd1doY05Nak13TWpBeE1USXpPREF3CldqQTlNUXN3Q1FZRFZRUUdFd0o0ZURFS01BZ0dBMVVFQ0JNQmVERUtNQWdHQTFVRUJ4TUJlREVLTUFnR0ExVUUKQ2hNQmVERUtNQWdHQTFVRUN4TUJlRENDQVNJd0RRWUpLb1pJaHZjTkFRRUJCUUFEZ2dFUEFEQ0NBUW9DZ2dFQgpBTkhrcU9tVmYyM0tNWGRhWlUyZUZVeDFoNHdiMDlKSU5CQjh4L0hMN1VFMEtGSmNuT29Wbk5RQjBnUnVrVW9wCmlZQ3pyek1GeUdXV21CL3BBRUtvb2wrWmlJMnVNeTZtY1lCRHRPaTRwT203VTBUUVFNVjZMLzVZZmk2NXhSejMKUlRNZC90WUFvRmk0YUNaYkpBR2p4VTZVV05ZRHpUeThFL2NQNlpubE5iVkhSaUE2L3dIc29XY1h0V1RYWVA1eQpuOWNmN0VXUWkxaE9CTTRCV21PSXlCMWY2TEVnUWlwWldNT01QUEhPM2hzdVNCbjByazdqb3ZTdDVYVGxiZ1JyCnR4cUFKaU5qSlV5a1d6SUYrbExuWkNpb2lwcEd2NXZrZEd2RTgzSm9BQ1h2WlRVd3pBK01MdTQ5Zmt3M2J3ZXEKa2JocmVyOGthY2pmR2x3M2FKTjM3ZUVDQXdFQUFhTkNNRUF3RGdZRFZSMFBBUUgvQkFRREFnRUdNQThHQTFVZApFd0VCL3dRRk1BTUJBZjh3SFFZRFZSME9CQllFRktYY2I1MmJ2Nm9xbkQrRDlmVE5GSFpMOElXeE1BMEdDU3FHClNJYjNEUUVCQ3dVQUE0SUJBUUFEdkt2djN5bTBYQVl3S3hQTExsM0xjNnNKWUhEYlROMGRvbmR1RzdQWGViMWQKaHV1a0oybGZ1ZlVZcDJJR1NBeHVMZWNUWWVlQnlPVnAxZ2FNYjVMc0lHdDJCVkRtbE1Na2lIMjlMVUhzdmJ5aQo4NUNwSm83QTVSSkc2QVdXMlZCQ2lEano1djhKRk02cE1rQlJGZlhIK3B3SWdlNjVDRStNVFNRY2ZiMS9hSUlvClEyMjZQN0UvM3VVR1g0azRwRFhHL083R052eWtGNDB2MURCNXk3RERCVFE0SldpSmZ5R2tUNjlUbWRPR0xGQW0Kand4VWpXeXZFZXk0cUpleC9FR0VtNVJRY012OWl5N3RiYTF3SzdzeWtOR241dURFTEdQR0lJRUFhNXJJSG0xRgpVRk9aWlZvRUxhYXNXUzU1OXd5OG9nMzlFcTIxZERNeW5iOEJuZG4vCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
`)
