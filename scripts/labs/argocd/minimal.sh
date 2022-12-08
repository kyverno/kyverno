#!/usr/bin/env bash

set -e

readonly SRC=$(dirname "$0")

$SRC/common-steps.sh

echo "---------------------------------------------------------------------------------"

ARGOCD_PASSWORD=$(kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d)

echo "ARGOCD is running and available at            http://localhost/argocd"
echo "- log in with admin / $ARGOCD_PASSWORD"
echo "PROMETHEUS is running and available at        http://localhost/prometheus"
echo "ALERTMANAGER is running and available at      http://localhost/alertmanager"
echo "GRAFANA is running and available at           http://localhost/grafana"
echo "- log in with admin / admin"
