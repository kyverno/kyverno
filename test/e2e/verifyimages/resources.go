package verifyimages

// Namespace Description
var namespaceYaml = []byte(`
apiVersion: v1
kind: Namespace
metadata:
  name: test-verify-images
`)

var tektonTaskCRD = []byte(`
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: tasks.tekton.dev
spec:
  group: tekton.dev
  preserveUnknownFields: false
  versions:
  - name: v1alpha1
    served: true
    storage: false
    schema:
      openAPIV3Schema:
        type: object
        x-kubernetes-preserve-unknown-fields: true
    subresources:
      status: {}
  - name: v1beta1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        x-kubernetes-preserve-unknown-fields: true
    subresources:
      status: {}
  - name: v1
    served: false
    storage: false
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
  conversion:
    strategy: Webhook
    webhook:
      conversionReviewVersions: ["v1beta1"]
      clientConfig:
        service:
          name: tekton-pipelines-webhook
          namespace: tekton-pipelines
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

var kyvernoTaskPolicyWithSimpleExtractor = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: tasks
spec:
  validationFailureAction: enforce
  rules:
  - name: verify-images
    match:
      resources:
        kinds:
        - Task
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
  name: tasks
spec:
  validationFailureAction: enforce
  rules:
  - name: verify-images
    match:
      resources:
        kinds:
        - Task
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
  name: tasks
spec:
  validationFailureAction: enforce
  webhookTimeoutSeconds: 30
  rules:
  - name: verify-images
    match:
      resources:
        kinds:
        - Task
    preconditions:
    - key: '{{request.operation}}'
      operator: NotEquals
      value: DELETE
    imageExtractors:
      Task:
        - path: /spec/steps/*/image
    verifyImages:
    - image: "ghcr.io/*"
      subject: "https://github.com/*"
      issuer: "https://token.actions.githubusercontent.com"
`)

var kyvernoTaskPolicyWithoutExtractor = []byte(`
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: tasks
spec:
  validationFailureAction: enforce
  rules:
  - name: verify-images
    match:
      resources:
        kinds:
        - Task
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
