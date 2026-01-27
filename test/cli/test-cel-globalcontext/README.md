# Testing CEL Policies with Context (No Cluster Required)

These tests demonstrate how to run Kyverno CLI tests for policies that use **CEL libraries** (e.g. `kyverno.globalcontext`, `kyverno.http`) **without a real Kubernetes cluster or live external services**.

## Problem statement

The CLI test framework supports real variable substitutions when CEL libraries are used. You provide fixture data in a **Context** file so that:

- **Global context** – `globalContext.Get(...)` in policies uses data from your context file.
- **HTTP calls** – `http.Get(...)` / `http.Post(...)` are stubbed; no real network calls are made.
- **Kubernetes API / resources** – Cluster resources and lookups can be faked via `spec.resources` in the context file.
- **Image data** – Image verification can use `spec.images` in the context file.

This enables fast, offline policy testing in CI/CD and on developer machines.

## How to run

From the repo root:

```bash
kyverno test test/cli/test-cel-globalcontext
kyverno test test/cli/test-cel-http
kyverno test test/cli/test-cel-combined
kyverno test test/cli/test-cel-globalcontext-file
kyverno test test/cli/test-cel-http-file
```

## Context file

In `kyverno-test.yaml`, set the `context` field to your context file:

```yaml
context: context.yaml
```

The context file must be `cli.kyverno.io/v1alpha1` `Context` with a `spec` that can include:

- **globalContext** – fixtures for `globalContext.Get(name, projection)` (inline `value` or `valueFile`).
- **http** – stubs for `http.Get`/`http.Post` (inline `body` or `bodyFile`, plus `method`, `url`, optional `status`, `headers`).
- **resources** – cluster-scoped resources for Kubernetes lookups.
- **images** – image metadata for image verification.

See the `context.yaml` in each test directory for examples.
