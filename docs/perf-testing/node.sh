#!/bin/bash

export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

# read user input for count
echo "Enter the count:"
read count

# iterate $count number of times
for (( i=1; i<=$count; i++ ))
do
  # generate YAML configuration using heredoc with COUNT variable substitution
  yaml=$(cat <<-END
    apiVersion: v1
    kind: Node
    metadata:
      annotations:
        node.alpha.kubernetes.io/ttl: "0"
        kwok.x-k8s.io/node: fake
      labels:
        beta.kubernetes.io/arch: amd64
        beta.kubernetes.io/os: linux
        kubernetes.io/arch: amd64
        kubernetes.io/hostname: kwok-node-$i
        kubernetes.io/os: linux
        kubernetes.io/role: agent
        node-role.kubernetes.io/agent: ""
        type: kwok
      name: kwok-node-$i
    spec:
      taints:
        - effect: NoSchedule
          key: kwok.x-k8s.io/node
          value: fake
    status:
      allocatable:
        cpu: 32
        memory: 256Gi
        pods: 110
      capacity:
        cpu: 32
        memory: 256Gi
        pods: 110
      nodeInfo:
        architecture: amd64
        bootID: ""
        containerRuntimeVersion: ""
        kernelVersion: ""
        kubeProxyVersion: fake
        kubeletVersion: fake
        machineID: ""
        operatingSystem: linux
        osImage: ""
        systemUUID: ""
      phase: Running
END
)

  # apply the generated configuration to Kubernetes cluster
  echo "$yaml" | kubectl apply -f -
done