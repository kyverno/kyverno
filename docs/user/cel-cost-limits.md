# CEL expression cost limits

Kyverno evaluates the CEL expressions in your policies inside its own controllers. To stop one expression from exhausting a controller, Kyverno meters the work each expression does and refuses expressions that cost too much.

This page explains the limits, what you see when an expression is refused, and which evaluation paths the limits cover.

## The limits

Kyverno applies the following limits. All of them are fixed in this release; there are no flags to change them.

| Limit | Value | Applies to |
| --- | --- | --- |
| Per-expression cost | 1,000,000 units | One CEL expression |
| Per-request cost | 10,000,000 units | All expressions evaluated for one admission request |
| Per-request cost, background | 3,000,000 units | All expressions evaluated for one background scan or background mutation |
| Argument size | 10 MB | The measured arguments of a single function call |

Cost units come from cel-go's own cost model. As a rough guide, a unit is about a tenth of a byte touched, so the per-expression limit permits roughly 10 MB of string and collection data per expression.

The background limit is lower than the admission limit because the background and reports controllers are configured with less memory than the admission controller. If you raise the memory limits for those controllers in the Helm chart, the cost limits do not change with them.

## What you see when an expression is refused

A refused expression produces a rule error, not a policy pass or fail. With `failurePolicy: Fail`, the request is rejected. The message names which limit was reached.

**`cel: per-program cost limit exceeded`**

One expression did more work than the per-expression limit allows. Reduce the size of the data the expression touches, or split the work across expressions.

**`cel: per-request cost budget exceeded`**

The expressions evaluated for this request added up past the per-request limit, even though each was individually within its own limit. This usually means a policy declares many expensive expressions, or several expensive policies matched the same request. The message reports the charge that exhausted the counter and how much remained.

**`cel: argument or guard limit exceeded`**

A single function call was refused before it ran, because its arguments predicted an output larger than the argument-size limit. The message names the function and the predicted size, for example:

```text
cel: join output bound 523239424 bytes, over the 10000000-byte cap:
cel: argument or guard limit exceeded
```

This applies to functions whose output can grow much larger than their input, including `join`, `format`, `replace`, `regex.replace`, `split`, `random`, `quantity`, `gzip.decompress`, `yaml.parse`, and `json.unmarshal`.

## Diagnosing a refusal

Refusals appear in the admission response and in the controller logs. To see them in the logs, raise the log level:

- Per-request budget refusals are logged at verbosity 2.
- Per-expression cost limit cancellations are logged at verbosity 4.

Both log the cost charged, the amount remaining, and the limit.

## Which paths are covered

The per-expression cost limit, the pricing that makes it effective, and the argument guards apply to **every** CEL evaluation path, including `ValidatingPolicy`, `MutatingPolicy`, `GeneratingPolicy`, `DeletingPolicy`, `ImageValidatingPolicy`, their `Namespaced` equivalents, `PolicyException` match conditions, and CEL expressions in `kyverno.io/v1` policies.

The per-request limit applies to:

- admission requests handled by the webhook,
- validating, mutating, and deleting policy engines,
- image verification,
- background mutation of existing resources,
- the reports scanner.

Generating policies are included. The generating engine establishes its own budget for the
generate scope, so a generating policy with many individually-cheap expressions is bounded in
total as well as per expression.

### Known limitation: CEL evaluated by the API server's own validator

CEL in `kyverno.io/v1` policies, and in ValidatingAdmissionPolicy and
MutatingAdmissionPolicy emulation, is compiled and evaluated by the API server's validator
rather than by Kyverno. Those evaluations carry the per-expression limit, the pricing that
makes it effective, and the argument guards, but **not** the per-request total: the API
server applies its own 10,000,000-unit budget, and it applies it fresh for each rule and
each parameter.

A policy with many parameterized rules therefore gets that budget once per rule instead of
once per request. Each individual evaluation is still bounded, so this limits the total a
single request can reach rather than leaving it unbounded. Closing it would require changing
how Kyverno calls the API server's validator, which is out of scope for this release.

## Effect on existing policies

An expression that does a genuinely large amount of work is now refused where it previously completed. Ordinary policies are unaffected: 42 common functions and the project's policy test corpus were checked with no refusals.

CEL expressions in namespaced `kyverno.io/v1` policies are now subject to these limits. Before this release they were evaluated without them. If you use CEL in namespaced `kyverno.io/v1` policies, test them against this release before upgrading a production cluster.
