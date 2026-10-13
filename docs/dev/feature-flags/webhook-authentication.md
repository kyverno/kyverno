# Experimental webhook authentication

`--webhookAuthentication=true` (or `FLAG_WEBHOOK_AUTHENTICATION=true`) makes the
admission and cleanup webhook servers require a Kubernetes-issued, webhook-bound
bearer token. The Helm setting is `features.webhookAuthentication.enabled`; both
controllers can override it through their `featuresOverride`. The default is
`false`.

This is receiver support for [KEP-6060](https://github.com/kubernetes/enhancements/tree/master/keps/sig-auth/6060-api-server-authentication-to-webhooks). It requires Kubernetes v1.37 with `APIServerWebhookAuthenticationToken` enabled for token issuance. The caller must request, refresh, and attach the token itself. Stock kube-apiserver does not yet attach these tokens to its normal admission webhook calls, so enabling this flag with stock callers will deny those calls. Disable the flag to restore the prior admission behavior.

Kyverno verifies the token signature, issuer, expiry, and exact audience using
the Kubernetes API server's discovery document and keys. The audience is the
configured Kyverno webhook service or URL plus the matched admission path.
Kyverno also checks the released `kubernetes.io.attestations.admissionReviewAPIGroups`
claim against the AdmissionReview resource group. Liveness and readiness probes
remain public. Token verification is initialized only when the feature is enabled.

This receiver does not check whether the bound webhook configuration or
ServiceAccount still exists. Deleting either object does not immediately revoke
an otherwise valid token at the receiver; token lifetime and Kubernetes signing
key rotation bound acceptance. This feature does not implement the API server's
automatic token acquisition, renewal, or presentation.

Related to [kyverno/kyverno#17399](https://github.com/kyverno/kyverno/issues/17399).
