<!-- coderabbit-ai-generated:v1 -->

# Webhook configuration reconciliation

Context: [Kyverno PR #17747](https://github.com/kyverno/kyverno/pull/17747) and [issue #17735](https://github.com/kyverno/kyverno/issues/17735).

Kyverno applies Kubernetes API defaults to the desired validating and mutating
webhook configurations before comparing them with the stored configurations.
Once the configurations settle, watchdog reconciliations skip update requests
when the managed settings already match. Explicit webhook settings are
preserved. The same comparison applies to the generation label protection
webhook when protection is enabled and `autoUpdateWebhooks=false`.

<!-- TODO(#17747): Add an operator troubleshooting note for repeated webhook
configuration writes. Verify the fix in API server audit logs across several
10-second watchdog ticks after the configuration settles. An unchanged
resourceVersion alone does not establish that no update request was sent. -->
