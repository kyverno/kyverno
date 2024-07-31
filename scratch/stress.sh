#!/bin/bash
# for i in {1..1000}
# do
# kubectl apply -f - <<EOF &
# apiVersion: v1
# kind: ConfigMap
# metadata:
#   name: testcase-${i}
# data:
#   foo: bar
# EOF
# done

while true
do

for i in {1..100}
do
timestamp=$(date +"%T")
kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: testcase-${i}
data:
  foo: bar-${timestamp}
EOF
done

# sleep 1
done
