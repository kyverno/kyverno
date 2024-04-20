FROM alpine:latest

ARG ARCH
ARG KUBECTL_VERSION=1.26.0

# Set the architecture based on the host machine
RUN case `uname -m` in \
    x86_64) ARCH=amd64; ;; \
    armv7l) ARCH=arm; ;; \
    aarch64) ARCH=arm64; ;; \
    ppc64le) ARCH=ppc64le; ;; \
    s390x) ARCH=s390x; ;; \
    *) echo "Unsupported architecture, exit ..."; exit 1; ;; \
    esac && \
    echo "ARCH=$ARCH" > /envfile

# Install required packages
RUN apk add --no-cache curl ca-certificates bash

# Install kubectl
RUN . /envfile && \
    curl -sLO https://storage.googleapis.com/kubernetes-release/release/v${KUBECTL_VERSION}/bin/linux/${ARCH}/kubectl && \
    mv kubectl /usr/bin/kubectl && \
    chmod +x /usr/bin/kubectl

WORKDIR /app
