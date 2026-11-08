# Namespaced legacy registry credential admission

Exercise `Policy` context and `verifyImages` registry credentials, and
`verifyImages` attestor `keys.secret` references, through deployed admission
webhooks. Bare, explicit local and templated namespace references must be
accepted; foreign and installation namespace references must fail with the scope
error. Only policies are created, so no image pull or external registry is needed.

```sh
chainsaw test --test-dir test/conformance/chainsaw/verify-images/namespaced-credential-scope
```

Runtime regression tests also exercise stored policies and variable substitution
before image verification, including a cached verification result.
