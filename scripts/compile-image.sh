#!/bin/bash

default_version="dev-testing"
version=$1

if [[ -z "$1" ]]
then 
    echo "Using default version: ${default_version}"
    version="${default_version}"
fi

hub_user_name="nirmata"
project_name="kyverno"

echo "# Ensuring Go dependencies..."
dep ensure || exit 2

echo "# Building executable ${project_name}..."
chmod +x scripts/update-codegen.sh
scripts/update-codegen.sh
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ${project_name} . || exit 3

echo "# Building docker image ${hub_user_name}/${project_name}:${version}"
cat <<EOF > Dockerfile
FROM alpine:latest
WORKDIR ~/
ADD ${project_name} ./${project_name}
ENTRYPOINT ["./${project_name}"]
EOF
tag="${hub_user_name}/${project_name}:${version}"
docker build --no-cache -t "${tag}" . || exit 4

echo "# Pushing image to repository..."
docker push "${tag}" || exit 5
