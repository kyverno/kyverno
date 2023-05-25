This document outlines the instructions for performance testing using [Kwok](https://kwok.sigs.k8s.io/) for the Kyverno 1.10 release.

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

To set up embedded etcd for the K3s cluster, follow instructions below. This is used when getting objects from etcd. 

```sh
k3d cluster create scaling --servers 3 --agents=15 --k3s-arg "--disable=metrics-server@server:*" --k3s-node-label "ingress-ready=true@agent:*"
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

## Variables preparation
```sh
KWOK_WORK_DIR=$(mktemp -d)
KWOK_REPO=kubernetes-sigs/kwok
KWOK_LATEST_RELEASE=$(curl "https://api.github.com/repos/${KWOK_REPO}/releases/latest" | jq -r '.tag_name')
```

## Render kustomization yaml
```sh
cat <<EOF > "${KWOK_WORK_DIR}/kustomization.yaml"
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
images:
  - name: registry.k8s.io/kwok/kwok
    newTag: "${KWOK_LATEST_RELEASE}"
resources:
  - "https://github.com/${KWOK_REPO}/kustomize/kwok?ref=${KWOK_LATEST_RELEASE}"
EOF
```

```sh
kubectl kustomize "${KWOK_WORK_DIR}" > "${KWOK_WORK_DIR}/kwok.yaml"
```

## `kwok` deployment 
```sh
kubectl apply -f "${KWOK_WORK_DIR}/kwok.yaml"
```

## Create `Kwok` nodes

Run the script to create the desired number of nodes for your Kowk cluster:

```sh
./node.sh
```

More about Kowk on this [page](https://kwok.sigs.k8s.io/docs/user/kwok-in-cluster/).

## Setup Monitor Components

```
make dev-lab-metrics-server dev-lab-prometheus
```

# Install Kyverno

```sh
helm repo update
helm upgrade --install kyverno kyverno/kyverno -n kyverno \
  --create-namespace \
  --devel \
  --set admissionController.serviceMonitor.enabled=true \
  --set reportsController.serviceMonitor.enabled=true \
  --set admissionController.replicas=3 \
  --set reportsController.resources.limits.memory=10Gi
```

## Deploy Kyverno PSS policies
```sh
helm upgrade --install kyverno kyverno/kyverno-policies --set=podSecurityStandard=restricted --set=background=true --set=validationFailureAction=Enforce --devel
```

# Create workloads

This script creates a single ReplicaSet with 1000 pods, with QPS and burst set to 50:

```sh
kubectl create ns test
go run release-1.10/main.go --count=1000 --kinds=pods --clientRateLimitQPS=50 --clientRateLimitBurst=50 --namespace=test
```

Note that these pods will be scheduled to the Kwok nodes, not k3s nodes.

# Prometheus Queries

## Memory utilization

You can select by the container image for a specific Kyverno controller:

```
container_memory_working_set_bytes{image="ghcr.io/kyverno/kyverno:v1.10.0-rc.1"}
```

`container_memory_working_set_bytes` gives you the current working set in bytes, and this is what the OOM killer is watching for.


## CPU utilization

```
rate(container_cpu_usage_seconds_total{image="ghcr.io/kyverno/kyverno:v1.10.0-rc.1"}[1m])
```


## Admission Request Rate

It's a bit tricky to get the precise Admission Request rate (ARPS). When using the Prometheus [rate()](https://prometheus.io/docs/prometheus/latest/querying/functions/#rate) function, it always requires a time window to calculate the rate with the given internal. The rate may differ when the window differs.


During our test, we calculate the increment in the count of admission requests recorded at the start and end time of a particular duration. Next, we divide this increment by the duration of the time window to derive the average admission request rate during that period.


```
sum(kyverno_admission_requests_total)
```

## Objects sizes in etcd

Run the following script to calculate total sizes for pods:
```sh
for key in `etcdctl get --prefix --keys-only /registry/pods`
do
  size=`etcdctl get $key --print-value-only | wc -c`
  count=`etcdctl get $key --write-out=fields | grep \"Count\" | cut -f2 -d':'`
  if [ $count -ne 0 ]; then
    versions=`etcdctl get $key --write-out=fields | grep \"Version\" | cut -f2 -d':'`
  else
    versions=0
  fi
  total=$(( $size * $versions))
  echo $total $size $versions $count $key >> /tmp/etcdkeys.txt
done
```