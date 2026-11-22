# AGENTS.md — pkg/event

This package provides centralized event generation and publishing for all Kyverno controllers (admission webhooks, background controllers, cleanup controllers, and report controllers), emitting standard Kubernetes `events.k8s.io/v1` `Event` resources asynchronously via a rate-limiting workqueue.

## ObjectReference Conventions: `Regarding` vs. `Related`

Every event struct (`Info`) contains two `corev1.ObjectReference` fields:
- **`Regarding` (Primary Subject)**:
  - For policy lifecycle, evaluation, background execution, and policy-exception events (`PolicyApplied`, `PolicyError`, `PolicySkipped`), `Regarding` is the **policy itself** (or the exception object).
  - For resource-level violations and creation notifications (`NewResourceViolationEvent`, `NewResourceGenerationEvent`, `NewFailedEvent`), `Regarding` points to the **target workload resource**.
  - If `Regarding.Namespace` is empty, the controller defaults the emitted event's namespace to `default` (cluster-scoped events are placed in `default`).
- **`Related` (Associated Subject)**:
  - Captures the secondary resource involved in the operation (e.g. the pod or deployment being evaluated or generated when `Regarding` references the policy).

## Dynamic APIVersion Resolution (Never Hardcode for Policy/Exception References)

- **Never hardcode literal API group/version strings** (such as `"kyverno.io/v1"` or `"kyverno.io/v2"`) when referencing policies or policy exceptions in `Regarding` or `Related`.
- Always derive versions dynamically via `policy.GetAPIVersion()`, `exception.GetAPIVersion()`, or `resource.APIVersion` for evaluated policies, exceptions, and workload resources.
- The `engineapi.GenericPolicy` and `engineapi.GenericException` interfaces abstract the union across legacy Kyverno types, CEL policies (`policies.kyverno.io/v1beta1`), and Kubernetes-native admission policies (`admissionregistration.k8s.io/v1`), dynamically providing the correct APIVersion for each policy kind.
- **Fixed Kubernetes Resource Kinds**: Fixed group-version strings are permitted only when referencing concrete standard Kubernetes resource kinds created directly by the controller (such as `"admissionregistration.k8s.io/v1"` for generated `ValidatingAdmissionPolicy` and `ValidatingAdmissionPolicyBinding` resources in `NewValidatingAdmissionPolicyEvent`).

## Event Name Sanitization & Kubernetes API Limits

- **RFC 1123 Subdomain Compliance**: `sanitizeEventName` strips or replaces non-compliant characters with hyphens and ensures the name starts and ends with an alphanumeric character.
- **253-Character Event Name Limit**: Event names are generated as `util.GenerateEventName(sanitizedName, t.UnixNano())`. The timestamp suffix occupies up to 17 characters (hyphen + 16-character timestamp), so base names must fit within 236 characters to prevent rejecting events exceeding the 253-character Kubernetes API server limit.
- **Message Truncation on Rune Boundaries**: Messages exceeding 1024 bytes are truncated at byte offset 1021 with trailing `...`. Truncation strictly walks back to the nearest UTF-8 rune start (`utf8.RuneStart`) to ensure multi-byte characters are never split.

## Filtering and Queuing Semantics

- **Success Event Gating**: `PolicyApplied` events are only enqueued if `cfg.GenerateSuccessEvents()` is enabled and the event's `Action` (`ResourcePassed`, `ResourceMutated`, `ResourceGenerated`, `ResourceCleanedUp`) is not excluded by configuration filters.
- **Omitted Events**: Individual events matching kinds specified in `--omit-events` are dropped at creation time before entering the workqueue.
- **Retry & Drop Handling**: Failed event creation requests are retried up to `workQueueRetryLimit` (3 attempts) before being dropped and recorded in event drop metrics.
