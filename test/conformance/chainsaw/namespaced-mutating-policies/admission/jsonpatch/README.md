This test verifies namespaced JSONPatch mutation on Deployment metadata. Its nginx
pods use `terminationGracePeriodSeconds: 0` so namespace cleanup does not wait for
the default 30-second pod termination grace period and exhaust Chainsaw's cleanup
deadline. Graceful application shutdown is not part of this test.
