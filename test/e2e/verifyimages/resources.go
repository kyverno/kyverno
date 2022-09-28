package verifyimages

import "fmt"

func newNamespaceYaml(name string) []byte {
	ns := fmt.Sprintf(`
  apiVersion: v1
  kind: Namespace
  metadata:
    name: %s
  `, name)
	return []byte(ns)
}

var tektonTaskCRD = []byte(`
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: tasks.tekton.dev
spec:
  group: tekton.dev
  preserveUnknownFields: false
  versions:
  - name: v1beta1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        x-kubernetes-preserve-unknown-fields: true
    subresources:
      status: {}
  names:
    kind: Task
    plural: tasks
    categories:
    - tekton
    - tekton-pipelines
  scope: Namespaced
`)

var tektonTask = []byte(`
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: example-task-name
spec:
  steps:
    - name: ubuntu-example
      image: ubuntu:bionic
`)

var tektonTaskVerified = []byte(`
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: example-task-name
spec:
  steps:
    - name: cosign
      image: ghcr.io/sigstore/cosign/cosign
`)

// not adding cosign.key and cosign.password as we only need cosign.pub
var secretResource = []byte(`
apiVersion: v1
kind: Secret
metadata:
  name: testsecret
  namespace: test-verify-images
data:
  cosign.pub: LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0KTUZrd0V3WUhLb1pJemowQ0FRWUlLb1pJemowREFRY0RRZ0FFOG5YUmg5NTBJWmJSajhSYS9OOXNicU9QWnJmTQo1L0tBUU4wL0tqSGNvcm0vSjV5Y3RWZDdpRWNuZXNzUlFqVTkxN2htS082SldWR0hwRGd1SXlha1pBPT0KLS0tLS1FTkQgUFVCTElDIEtFWS0tLS0t
type: Opaque
`)

var secretPodResourceSuccess = []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: test-secret-pod
  namespace: test-verify-images
spec:
  containers:
  - image: ghcr.io/kyverno/test-verify-image:signed
    name: test-secret
`)

var secretPodResourceFailed = []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: test-secret-pod
  namespace: test-verify-images
spec:
  containers:
  - image: ghcr.io/kyverno/test-verify-image:unsigned
    name: test-secret
`)

var kyvernoPolicyWithSecretInKeys = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: secret-in-keys
spec:
  validationFailureAction: enforce
  background: false
  webhookTimeoutSeconds: 30
  failurePolicy: Fail
  rules:
  - name: check-secret-in-keys
    match:
      resources:
        kinds:
        - Pod
    verifyImages:
    - imageReferences:
      - "ghcr.io/kyverno/test-verify-image:*"
      attestors:
      - entries:
        - keys:
            secret:
              name: testsecret
              namespace: test-verify-images
`)

var kyvernoTaskPolicyWithSimpleExtractor = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: tasks-simple
spec:
  validationFailureAction: enforce
  rules:
  - name: verify-images
    match:
      resources:
        kinds:
        - tekton.dev/v1beta1/Task
    preconditions:
    - key: '{{request.operation}}'
      operator: NotEquals
      value: DELETE
    imageExtractors:
      Task:
        - path: /spec/steps/*/image
    verifyImages:
    - image: "*"
      key: |-
        -----BEGIN PUBLIC KEY-----
        MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM
        5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==
        -----END PUBLIC KEY----- 
`)

var kyvernoTaskPolicyWithComplexExtractor = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: tasks-complex
spec:
  validationFailureAction: enforce
  rules:
  - name: verify-images
    match:
      resources:
        kinds:
        - tekton.dev/v1beta1/Task
    preconditions:
    - key: '{{request.operation}}'
      operator: NotEquals
      value: DELETE
    imageExtractors:
      Task:
        - path: /spec/steps/*
          name: steps
          value: image
          key: name
    verifyImages:
    - image: "*"
      key: |-
        -----BEGIN PUBLIC KEY-----
        MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM
        5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==
        -----END PUBLIC KEY----- 
`)

var kyvernoTaskPolicyKeyless = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: tasks-keyless
spec:
  validationFailureAction: enforce
  webhookTimeoutSeconds: 30
  rules:
  - name: verify-images
    match:
      resources:
        kinds:
        - tekton.dev/v1beta1/Task
    preconditions:
    - key: '{{request.operation}}'
      operator: NotEquals
      value: DELETE
    imageExtractors:
      Task:
        - path: /spec/steps/*/image
    verifyImages:
    - imageReferences:
      - "ghcr.io/*"
      attestors:
      - count: 1
        entries:
        - keyless:
            issuer: "https://token.actions.githubusercontent.com"
            subject: "https://github.com/*"
            rekor:
              url: https://rekor.sigstore.dev
      required: false
`)

var kyvernoTaskPolicyKeylessRequired = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: tasks-keyless-required
spec:
  validationFailureAction: enforce
  webhookTimeoutSeconds: 30
  rules:
  - name: verify-images
    match:
      resources:
        kinds:
        - tekton.dev/v1beta1/Task
    preconditions:
    - key: '{{request.operation}}'
      operator: NotEquals
      value: DELETE
    imageExtractors:
      Task:
        - path: /spec/steps/*/image
    verifyImages:
    - imageReferences:
      - "ghcr.io/*"
      attestors:
      - count: 1
        entries:
        - keyless:
            issuer: "https://token.actions.githubusercontent.com"
            subject: "https://github.com/*"
            rekor:
              url: https://rekor.sigstore.dev
      required: true
`)

var kyvernoTaskPolicyWithoutExtractor = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: tasks-no-extractor
spec:
  validationFailureAction: enforce
  rules:
  - name: verify-images
    match:
      resources:
        kinds:
        - tekton.dev/v1beta1/Task
    preconditions:
    - key: '{{request.operation}}'
      operator: NotEquals
      value: DELETE
    verifyImages:
    - image: "*"
      key: |-
        -----BEGIN PUBLIC KEY-----
        MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM
        5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==
        -----END PUBLIC KEY----- 
`)

var cpolVerifyImages = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: verify-images
spec:
  validationFailureAction: enforce
  rules:
    - name: check-image-sig
      match:
        any:
        - resources:
            kinds:
              - Pod
      verifyImages:
      - image: "harbor2.zoller.com/cosign/*"
        mutateDigest: false
        verifyDigest: false
        required: false
        key: |-
          -----BEGIN PUBLIC KEY-----
          MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEpNlOGZ323zMlhs4bcKSpAKQvbcWi
          5ZLRmijm6SqXDy0Fp0z0Eal+BekFnLzs8rUXUaXlhZ3hNudlgFJH+nFNMw==
          -----END PUBLIC KEY-----
`)
