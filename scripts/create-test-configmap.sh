cd "$(dirname "$0")"
kubectl create -f resources/test-configmap.yaml
kubectl delete -f resources/test-configmap.yaml
