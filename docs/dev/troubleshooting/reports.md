# Reports

This document contains scripts to help troubleshooting reports issues.

## Getting reports detailed information

When querying reports you can add `-o wide` to get a more detailed output.

This will show infos about the resource associated with the report.

It can be useful to determine if a particular resource kind is responsible for creating too many reports.

If `APIVERSION`, `KIND` and `SUBJECT` is empty it means the report is orphan and it is an issue if the report is more than a couple minutes old.

```console
# list cluster admission reports
kubectl get cadmr -o wide

# list cluster background scan reports
kubectl get cbgscanr -o wide

# list admission reports
kubectl get admr -A -o wide

# list background scan reports
kubectl get bgscanr -A -o wide
```

Below is an example of the output:

```console
$ kubectl get cadmr -o wide
NAME                                   APIVERSION                     KIND                       SUBJECT                                            PASS   FAIL   WARN   ERROR   SKIP   AGE     HASH
06aa537a-e81d-4253-8eb2-cd72f366a000   rbac.authorization.k8s.io/v1   ClusterRole                view                                               0      0      0      0       1      8h      5c71e236747d42e7b2a0cedd3f36434d
076d70b7-64c4-41b0-957e-07122680f930   apiextensions.k8s.io/v1        CustomResourceDefinition   generaterequests.kyverno.io                        0      0      0      0       1      7h48m   b82b99dd89e7ed7ec064d2f96d4b690a
```

## Getting the number of reports in a cluster

This will help checking if reports are incorrectly accumulating in the cluster.

```console
COUNT=$(kubectl get cadmr --no-headers 2> /dev/null | wc -l)
echo "number of cluster admission reports: $COUNT"

COUNT=$(kubectl get cbgscanr --no-headers 2> /dev/null | wc -l)
echo "number of cluster background scan reports: $COUNT"

COUNT=$(kubectl get admr -A --no-headers 2> /dev/null | wc -l)
echo "number of admission reports: $COUNT"

COUNT=$(kubectl get bgscanr -A --no-headers 2> /dev/null | wc -l)
echo "number of background scan reports: $COUNT"

NS_LIST=$(kubectl get ns -o jsonpath='{.items[*].metadata.name}')
for ns in $NS_LIST
do
    COUNT=$(kubectl get -n $ns admr --no-headers 2> /dev/null | wc -l)
    echo "number of admission reports in $ns: $COUNT"

    COUNT=$(kubectl get -n $ns bgscanr --no-headers 2> /dev/null | wc -l)
    echo "number of background scan reports in $ns: $COUNT"
done
```

## Getting the number of reports per kind

Use the script below to get number of reports per resource kind in a cluster.

This will help determining if a particular resource kind is responsible for creating too many reports.

```console
API_LIST=$(kubectl api-resources --namespaced=false --no-headers | awk '{print $NF}')
for api in $API_LIST
do
    COUNT=$(kubectl get cadmr --no-headers -o jsonpath="{range .items[?(@.metadata.ownerReferences[0].kind=='$api')]}{.metadata.name}{'\n'}{end}" 2> /dev/null | wc -l)
    echo "number of cluster admission reports for $api: $COUNT"

    COUNT=$(kubectl get cbgscanr --no-headers -o jsonpath="{range .items[?(@.metadata.ownerReferences[0].kind=='$api')]}{.metadata.name}{'\n'}{end}" 2> /dev/null | wc -l)
    echo "number of cluster background scan reports for $api: $COUNT"
done

API_LIST=$(kubectl api-resources --namespaced=true --no-headers | awk '{print $NF}')
for api in $API_LIST
do
    COUNT=$(kubectl get admr -A --no-headers -o jsonpath="{range .items[?(@.metadata.ownerReferences[0].kind=='$api')]}{.metadata.name}{'\n'}{end}" 2> /dev/null | wc -l)
    echo "number of admission reports for $api: $COUNT"

    COUNT=$(kubectl get bgscanr --no-headers -o jsonpath="{range .items[?(@.metadata.ownerReferences[0].kind=='$api')]}{.metadata.name}{'\n'}{end}" 2> /dev/null | wc -l)
    echo "number of background scan reports for $api: $COUNT"    
done
```

## Watching report changes

By using `--watch-only` with `kubectl` you can view report changes only without first listing existing reports.

Listing existing reports can take a long time when there is a high number of reports.

With `--watch-only` you only get an output for reports that are created, updated or deleted.

This is useful to determine if particular resource kind is reponsible for creating too many reports.

```console
# watch changing cluster admission reports
kubectl get cadmr -o wide -w --watch-only

# watch changing cluster background scan reports
kubectl get cbgscanr -o wide -w --watch-only

# watch changing admission reports
kubectl get admr -A -o wide -w --watch-only

# watch changing background scan reports
kubectl get bgscanr -A -o wide -w --watch-only
```

## Getting orphan reports count

Orphan reports can exist in a cluster but should stay pretty low.

Orphan reports will be either adopted or deleted.

A high number of orphan reports indicates that something is not working correctly.

```console
ALL=$(kubectl get cadmr --no-headers | wc -l)
NOT_ORPHANS=$(kubectl get cadmr --no-headers -o jsonpath="{range .items[?(@.metadata.ownerReferences[0].uid)]}{.metadata.name}{'\n'}{end}" 2> /dev/null | wc -l)
echo "number of orphan cluster admission reports: $((ALL-NOT_ORPHANS)) ($ALL - $NOT_ORPHANS)"

ALL=$(kubectl get cadmr --no-headers 2> /dev/null | wc -l)
NOT_ORPHANS=$(kubectl get cadmr --no-headers -o jsonpath="{range .items[?(@.metadata.ownerReferences[0].uid)]}{.metadata.name}{'\n'}{end}" 2> /dev/null | wc -l)
echo "number of orphan cluster background scan reports: $((ALL-NOT_ORPHANS)) ($ALL - $NOT_ORPHANS)"

ALL=$(kubectl get admr -A --no-headers 2> /dev/null | wc -l)
NOT_ORPHANS=$(kubectl get admr -A --no-headers -o jsonpath="{range .items[?(@.metadata.ownerReferences[0].uid)]}{.metadata.name}{'\n'}{end}" 2> /dev/null | wc -l)
echo "number of orphan admission reports: $((ALL-NOT_ORPHANS)) ($ALL - $NOT_ORPHANS)"

ALL=$(kubectl get bgscanr -A --no-headers 2> /dev/null | wc -l)
NOT_ORPHANS=$(kubectl get bgscanr -A --no-headers -o jsonpath="{range .items[?(@.metadata.ownerReferences[0].uid)]}{.metadata.name}{'\n'}{end}" 2> /dev/null | wc -l)
echo "number of orphan background scan reports: $((ALL-NOT_ORPHANS)) ($ALL - $NOT_ORPHANS)"
```
