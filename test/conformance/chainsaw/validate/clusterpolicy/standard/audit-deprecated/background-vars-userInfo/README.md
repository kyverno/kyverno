# Title

Ensures this policy cannot be created because clusterRoles is not valid in background mode. It checks that the return failure output contains the given string and finally checks that the policy has not been created (in case somehow it returned an error, which passed, but was still created).