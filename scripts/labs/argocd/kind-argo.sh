#!/usr/bin/env bash

set -e

readonly SRC=$(dirname "$0")

. $SRC/0-cluster.sh
. $SRC/1-ingress-nginx.sh
. $SRC/2-argocd.sh
. $SRC/3-kube-prometheus-stack.sh
. $SRC/4-metrics-server.sh
. $SRC/5-loki.sh
. $SRC/6-tempo.sh
. $SRC/7-kyverno.sh
. $SRC/8-kyverno-policies.sh
. $SRC/9-policy-reporter.sh

echo "---------------------------------------------------------------------------------"

ARGOCD_PASSWORD=$(kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d)

echo "ARGOCD is running and available at            http://localhost/argocd"
echo "- log in with admin / $ARGOCD_PASSWORD"
echo "POLICY-REPORTER is running and available at   http://localhost/policy-reporter"
echo "PROMETHEUS is running and available at        http://localhost/prometheus"
echo "ALERTMANAGER is running and available at      http://localhost/alertmanager"
echo "GRAFANA is running and available at           http://localhost/grafana"
echo "- log in with admin / admin"
