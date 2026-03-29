# CEL Expression Support for Identity Fields

## Overview

`ImageValidatingPolicy` and `NamespacedImageValidatingPolicy` now support CEL
expressions in the `subject` and `subjectRegExp` fields of keyless cosign
identities. This allows a single policy to dynamically match signing identities
based on the image reference being verified, eliminating the need for one policy
per image.

Previously only static strings were accepted. Other fields like `cert` and
`certChain` already used `StringOrExpression`; this change brings `subject` and
`subjectRegExp` to the same level.

## API Change (kyverno/api#64)

`Identity.Subject` and `Identity.SubjectRegExp` changed from `string` to
`*StringOrExpression`:

```go
// Before
type Identity struct {
    Subject       string `json:"subject,omitempty"`
    SubjectRegExp string `json:"subjectRegExp,omitempty"`
    // ...
}

// After
type Identity struct {
    Subject       *StringOrExpression `json:"subject,omitempty"`
    SubjectRegExp *StringOrExpression `json:"subjectRegExp,omitempty"`
    // ...
}
```

`StringOrExpression` holds either a static `value` or a CEL `expression`:

```go
type StringOrExpression struct {
    Value      string `json:"value,omitempty"`
    Expression string `json:"expression,omitempty"`
}
```

## CEL Context

The `image` variable (type `string`) is available in identity expressions. It
contains the full image reference being verified, e.g.
`ghcr.io/myorg/myrepo:v1.0.0`.

All standard CEL string extensions (`split`, `join`, `replace`, etc.) are
available.

## Example Policies

### Static subject (backward compatible)

```yaml
apiVersion: policies.kyverno.io/v1beta1
kind: ImageValidatingPolicy
metadata:
  name: verify-static-identity
spec:
  attestors:
    - name: cosign-keyless
      cosign:
        keyless:
          identities:
            - issuer: https://token.actions.githubusercontent.com
              subject:
                value: https://github.com/myorg/myrepo/.github/workflows/release.yml@refs/heads/main
```

### Dynamic subject from image reference

```yaml
apiVersion: policies.kyverno.io/v1beta1
kind: ImageValidatingPolicy
metadata:
  name: verify-dynamic-identity
spec:
  matchImageReferences:
    - glob: "ghcr.io/myorg/*"
  attestors:
    - name: cosign-keyless
      cosign:
        keyless:
          identities:
            - issuer: https://token.actions.githubusercontent.com
              subject:
                # image = "ghcr.io/myorg/myrepo:v1.0.0"
                # image.split("/")[1] = "myorg"
                # image.split("/")[2] = "myrepo:v1.0.0"
                expression: >-
                  "https://github.com/" + image.split("/")[1] + "/" +
                  image.split("/")[2].split(":")[0] +
                  "/.github/workflows/release.yml@refs/heads/main"
  validations:
    - expression: "images.containers.map(image, verifyImageSignatures(image, [attestors.cosign-keyless])).all(e, e > 0)"
      message: "image signature verification failed"
```

### Dynamic subjectRegExp

```yaml
apiVersion: policies.kyverno.io/v1beta1
kind: ImageValidatingPolicy
metadata:
  name: verify-dynamic-regexp
spec:
  matchImageReferences:
    - glob: "ghcr.io/myorg/*"
  attestors:
    - name: cosign-keyless
      cosign:
        keyless:
          identities:
            - issuer: https://token.actions.githubusercontent.com
              subjectRegExp:
                # Matches any workflow in the repo derived from the image name.
                expression: >-
                  "https://github\\.com/" + image.split("/")[1] + "/" +
                  image.split("/")[2].split(":")[0] + "/.*"
  validations:
    - expression: "images.containers.map(image, verifyImageSignatures(image, [attestors.cosign-keyless])).all(e, e > 0)"
      message: "image signature verification failed"
```

## Migration Notes

Existing policies using plain string values continue to work unchanged. The
`value` field of `StringOrExpression` is a drop-in replacement:

```yaml
# Old (still valid via JSON unmarshalling if API supports it)
subject: "https://github.com/org/repo/..."

# New equivalent
subject:
  value: "https://github.com/org/repo/..."
```

## Implementation Notes

- Expressions are compiled at policy admission time via `CompileAttestorIdentities`.
- Evaluation happens per-image inside `verifyImageSignatures` / `verifyAttestationSignatures` CEL functions.
- The `image` variable is the full image reference string passed to the CEL function.
- If an expression fails to evaluate at runtime, the verification call returns an error.
