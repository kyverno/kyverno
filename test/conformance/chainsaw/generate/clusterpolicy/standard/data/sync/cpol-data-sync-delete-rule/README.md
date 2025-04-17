## Description

This test checks to ensure that deletion of a rule in a ClusterPolicy generate rule, data declaration, with sync enabled, results in the downstream resource's deletion.

## Expected Behavior

1. create the namespace that triggers the policy, two rules should be applied and generate a configmap and a secret correspondingly.
2. update the configmap rule and trigger the policy again, a new configmap should be generated.
3. delete the newly updated configmap rule, the new configmap should be deleted while the old configmap preserves.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/5744
