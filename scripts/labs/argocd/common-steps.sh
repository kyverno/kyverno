#!/usr/bin/env bash

set -e

readonly SRC=$(dirname "$0")

$SRC/0-cluster.sh
$SRC/1-ingress-nginx.sh
$SRC/2-argocd.sh
$SRC/3-kube-prometheus-stack.sh
$SRC/4-loki.sh
$SRC/5-tempo.sh
