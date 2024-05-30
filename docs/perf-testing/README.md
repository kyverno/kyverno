This document outlines the instructions for performance testing using [Kwok](https://kwok.sigs.k8s.io/) for the Kyverno 1.12 release.

# Pre-requisite

## Install etcdctl

```sh
ETCD_VER=v3.4.13

# choose either URL
GOOGLE_URL=https://storage.googleapis.com/etcd
GITHUB_URL=https://github.com/etcd-io/etcd/releases/download
DOWNLOAD_URL=${GOOGLE_URL}

rm -f /tmp/etcd-${ETCD_VER}-linux-amd64.tar.gz
rm -rf /tmp/etcd-download-test && mkdir -p /tmp/etcd-download-test

curl -L ${DOWNLOAD_URL}/${ETCD_VER}/etcd-${ETCD_VER}-linux-amd64.tar.gz -o /tmp/etcd-${ETCD_VER}-linux-amd64.tar.gz
tar xzvf /tmp/etcd-${ETCD_VER}-linux-amd64.tar.gz -C /usr/local/bin --strip-components=1
rm -f /tmp/etcd-${ETCD_VER}-linux-amd64.tar.gz

etcd --version
etcdctl version
```

More details for etcdctl installation can be found [here](https://github.com/etcd-io/etcd/releases/tag/v3.4.13).

## Download k3d:
```sh
wget -q -O - https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash
```

More details for k3d installation can be found [here](https://k3d.io/v5.4.9/#install-script).

# Create a base cluster using K3d

To quickly try out the scaling test, you can use the following command to create the k3d cluster with 3 workers:
```sh
k3d cluster create --agents=3  --k3s-arg "--disable=metrics-server@server:*" --k3s-node-label "ingress-ready=true@agent:*"
```

To set up embedded etcd for the K3s cluster, follow instructions below.

```sh
k3d cluster create scaling --servers 3 --agents=15 --k3s-arg "--disable=metrics-server@server:*" --k3s-node-label "ingress-ready=true@agent:*" 
```

Use the following command if you want to configure the etcd storage limit, this command sets the storage limit to 8GB:
```sh
k3d cluster create scaling --servers 3 --agents=15 --k3s-arg "--disable=metrics-server@server:*" --k3s-node-label "ingress-ready=true@agent:*" --k3s-arg "--etcd-arg=quota-backend-bytes=8589934592@server:*"
```

Note, you can execute into the server node to check the storage setting:
```
docker exec -ti k3d-scaling-server-0 sh
cat /var/lib/rancher/k3s/server/db/etcd/config | tail -2
quota-backend-bytes: 8589934592
```

## Prepare etcd access

```sh
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
docker cp k3d-scaling-server-0:/var/lib/rancher/k3s/server/tls/etcd/server-ca.crt ./server-ca.crt
docker cp k3d-scaling-server-0:/var/lib/rancher/k3s/server/tls/etcd/server-client.crt ./server-client.crt
docker cp k3d-scaling-server-0:/var/lib/rancher/k3s/server/tls/etcd/server-client.key ./server-client.key
etcd=https://$(kubectl get node -o wide | grep k3d-scaling-server-0 | awk '{print $6}'):2379
etcd_ep=$etcd/version
curl -L --cacert ./server-ca.crt --cert ./server-client.crt --key ./server-client.key $etcd_ep
export ETCDCTL_ENDPOINTS=$etcd
export ETCDCTL_CACERT='./server-ca.crt'
export ETCDCTL_CERT='./server-client.crt'
export ETCDCTL_KEY='./server-client.key'
export ETCDCTL_API=3

etcdctl endpoint status -w table
```

Credits to [k3s etcd commands](https://gist.github.com/superseb/0c06164eef5a097c66e810fe91a9d408).

# Deploy Kwok in a cluster

```sh
./docs/perf-testing/kwok.sh
```

## Create `Kwok` nodes

Run the script to create the desired number of nodes for your Kwok cluster:

```sh
./docs/perf-testing/node.sh
```

More about Kwok on this [page](https://kwok.sigs.k8s.io/docs/user/kwok-in-cluster/).

## Setup Monitor Components

```
make dev-lab-metrics-server dev-lab-prometheus
```

# Install Kyverno

```sh
helm repo update
helm upgrade --install kyverno kyverno/kyverno -n kyverno \
  --create-namespace \
  --set admissionController.serviceMonitor.enabled=true \
  --set admissionController.replicas=3 \
  --set reportsController.serviceMonitor.enabled=true \
  --set reportsController.resources.limits.memory=10Gi \
  --set "features.omitEvents.eventTypes={PolicyApplied,PolicySkipped,PolicyViolation,PolicyError}" \
  # --devel \
  # --set features.admissionReports.enabled=false \
```

## Deploy Kyverno PSS policies
```sh
helm upgrade --install kyverno kyverno/kyverno-policies --set=podSecurityStandard=restricted --set=background=true --set=validationFailureAction=Audit --devel
```

# Testing the reports controller

The following instructions provide steps to create policyreports for installed workloads, measure resource usages of the reports controller and the total objects size in etcd.

## Create workloads

This script creates 100 deployments in namespace `test-1`, each deployment has 10 replicas:

```
./docs/perf-testing/deployment.sh
Enter the deployment count:
100
Enter the deployment replicas:
10
Enter the deployment namespace:
test-1
Creating namespace test-1
...
```

The total number of policyreports for the 100 deployments with 10 replicas each is 1200. With Kyverno 1.12.0, a policy report is created for one matching resource, therefore 100 deployments, 100 replicasets and 1000 pods will create 1200 policy reports in total.

You can also create pods directly using `./docs/perf-testing/pod.sh`.

Note that these pods will be scheduled to the Kwok nodes, not K3d nodes.

## Objects sizes in etcd

Run the following script to calculate total sizes for the given resource (policyreports in the following example):
```sh
$ ./docs/perf-testing/size.sh
Enter the resource to caclutate the size:
wgpolicyk8s.io/policyreports
The total size for wgpolicyk8s.io/policyreports is 401851071 bytes.
```

You can also check the total etcd size:
```sh
$ etcdctl endpoint status -w table
+-------------------------+------------------+---------+---------+-----------+------------+-----------+------------+--------------------+--------+
|        ENDPOINT         |        ID        | VERSION | DB SIZE | IS LEADER | IS LEARNER | RAFT TERM | RAFT INDEX | RAFT APPLIED INDEX | ERRORS |
+-------------------------+------------------+---------+---------+-----------+------------+-----------+------------+--------------------+--------+
| https://172.21.0.2:2379 | c2ed0eb8fc7bc4fc |   3.5.9 |  1.8 GB |      true |      false |         2 |    2428629 |            2428629 |        |
+-------------------------+------------------+---------+---------+-----------+------------+-----------+------------+--------------------+--------+
```

This command returns the resources stored in etcd that have more than 100 objects:

```sh
kubectl get --raw=/metrics | grep apiserver_storage_objects |awk '$2>100' |sort -g -k 2
```

# Prometheus Queries

To view the Prometheus dashboard, you can expose it on your localhost's port at 9090:
```
kubectl port-forward --address 127.0.0.1 svc/kube-prometheus-stack-prometheus 9090:9090 -n monitoring &
```

## Memory utilization

To get an view of the memory utilization overtime, you can select by the container image for a specific Kyverno controller:

```
container_memory_working_set_bytes{image="ghcr.io/kyverno/kyverno:v1.12.0-rc.5"}
```

`container_memory_working_set_bytes` gives you the current working set in bytes, and this is what the OOM killer is watching for.


## CPU utilization

```
rate(container_cpu_usage_seconds_total{image="ghcr.io/kyverno/kyverno:v1.12.0-rc.5"}[1m])
```

`container_cpu_usage_seconds_total` is the sum of the total amount of “user” time (i.e. time spent not in the kernel) and the total amount of “system” time (i.e. time spent in the kernel). This query gives the average CPU usage in the last 1 minute.