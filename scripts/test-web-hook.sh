#!/bin/bash
# You should see the trace of requests in the output of webhook server
kubectl create configmap test-config-map --from-literal="some_var=some_value"
kubectl delete configmap test-config-map
