---
title: Home
slug: /docs/index
description: Kyverno is a policy engine designed for Kubernetes.
---

# Kyverno - Kubernetes Native Policy Management

[![Build Status](https://travis-ci.org/nirmata/kyverno.svg?branch=master)](https://travis-ci.org/nirmata/kyverno) [![Go Report Card](https://goreportcard.com/badge/github.com/nirmata/kyverno)](https://goreportcard.com/report/github.com/nirmata/kyverno)


Kubernetes supports declarative validation, mutation, and generation of resource configurations using policies written as Kubernetes resources. 

Kyverno can be used to scan existing workloads for best practices, or can be used to enforce best practices by blocking or mutating API requests.Kyverno allows cluster adminstrators to manage environment specific configurations independently of workload configurations and enforce configuration best practices for their clusters.

Kyverno policies are Kubernetes resources that can be written in YAML or JSON. Kyverno policies can validate, mutate, and generate any Kubernetes resources.

Kyverno runs as a [dynamic admission controller](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) in a Kubernetes cluster. Kyverno receives validating and mutating admission webhook HTTP callbacks from the kube-apiserver and applies matching policies to return results that enforce admission policies or reject requests.

Kyverno policies can match resources using the resource kind, name, and label selectors. Wildcards are supported in names.

Mutating policies can be written as overlays (similar to [Kustomize](https://kubernetes.io/docs/tasks/manage-kubernetes-objects/kustomization/#bases-and-overlays)) or as a [JSON Patch](http://jsonpatch.com/). Validating policies also use an overlay style syntax, with support for pattern matching and conditional (if-then-else) processing.

Policy enforcement is captured using Kubernetes events. Kyverno also reports policy violations for existing resources.
