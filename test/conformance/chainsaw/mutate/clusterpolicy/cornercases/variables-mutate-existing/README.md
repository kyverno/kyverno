## Description

This test checks that variable `request.object` always references the admission request object data in mutateExisting policies.

## Expected Behavior

With the mutateExisting policy, the variable `request.object` should always be substituted to the matching configmap's name `mycm`, not any pod's name. When the test finishes, the annotation `corp.org/random=bar` should be added to the pod `foo/mypod`.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/5820