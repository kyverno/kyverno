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



export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
kubectl create ns test
go run docs/perf-testing/main.go --count=10000 --kinds=pods --clientRateLimitQPS=50 --clientRateLimitBurst=50 --namespace=test-1



 TYPE apiserver_storage_objects gauge
apiserver_storage_objects{resource="nodes"} 818
apiserver_storage_objects{resource="policyreports.wgpolicyk8s.io"} 1193
apiserver_storage_objects{resource="backgroundscanreports.kyverno.io"} 64370
apiserver_storage_objects{resource="pods"} 80856
apiserver_storage_objects{resource="events"} 92177
root@da-m3-large-x86-02:~/go/src/github/realshuting/kyverno# etcdctl endpoint status -w table
+--------------------------+------------------+---------+---------+-----------+------------+-----------+------------+--------------------+--------+
|         ENDPOINT         |        ID        | VERSION | DB SIZE | IS LEADER | IS LEARNER | RAFT TERM | RAFT INDEX | RAFT APPLIED INDEX | ERRORS |
+--------------------------+------------------+---------+---------+-----------+------------+-----------+------------+--------------------+--------+
| https://192.168.0.2:2379 | 78243b7276c832d5 |   3.5.3 |  2.4 GB |      true |      false |         2 |     606948 |             606948 |        |
+--------------------------+------------------+---------+---------+-----------+------------+-----------+------------+--------------------+--------+
root@da-m3-large-x86-02:~/go/src/github/realshuting/kyverno#


/ # ls -lh /var/lib/rancher/k3s/server/db/etcd-tmp/member/snap/db
-rw------- 1 0 0 2.2G May 29 08:44 /var/lib/rancher/k3s/server/db/etcd-tmp/member/snap/db