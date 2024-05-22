#!/bin/bash

export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

# read user input for count
echo "Enter the deployment count:"
read count

echo "Enter the deployment replicas:"
read replicas

echo "Enter the deployment namespace:"
read namespace

echo "Creating namespace $namespace:"
kubectl create namespace $namespace

# iterate $count number of times
for (( i=1; i<=$count; i++ ))
do
  # generate YAML configuration using heredoc with COUNT variable substitution
  yaml=$(cat <<-END
apiVersion: apps/v1
kind: Deployment
metadata:
  name: fake-pod-$i
  namespace: $namespace
spec:
  replicas: $replicas
  selector:
    matchLabels:
      app: fake-pod
  template:
    metadata:
      labels:
        app: fake-pod
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: type
                operator: In
                values:
                - kwok
      # A taints was added to an automatically created Node.
      # You can remove taints of Node or add this tolerations.
      tolerations:
      - key: "kwok.x-k8s.io/node"
        operator: "Exists"
        effect: "NoSchedule"
      containers:
      - name: fake-container
        image: fake-image
END
)

  # apply the generated configuration to Kubernetes cluster
  echo "$yaml" | kubectl apply -f -
done
