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
    - image: "ghcr.io/*"
      subject: "https://github.com/*"
      issuer: "https://token.actions.githubusercontent.com"
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
    - image: "ghcr.io/*"
      subject: "https://github.com/*"
      issuer: "https://token.actions.githubusercontent.com"
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
