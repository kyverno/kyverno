FROM golang:1.13-alpine

LABEL maintainer="MinIO Inc <dev@min.io>"

ENV GOPATH /go
ENV CGO_ENABLED 0
ENV GO111MODULE on
ENV GOPROXY https://proxy.golang.org

RUN  \
     apk add --no-cache git && \
     git clone https://github.com/minio/minio && cd minio && \
     go install -v -ldflags "$(go run buildscripts/gen-ldflags.go)"

FROM alpine:3.9

ENV MINIO_UPDATE off
ENV MINIO_ACCESS_KEY_FILE=access_key \
    MINIO_SECRET_KEY_FILE=secret_key \
    MINIO_KMS_MASTER_KEY_FILE=kms_master_key

EXPOSE 9000

COPY --from=0 /go/bin/minio /usr/bin/minio
COPY dockerscripts/docker-entrypoint.sh /usr/bin/

RUN  \
     apk add --no-cache ca-certificates 'curl>7.61.0' 'su-exec>=0.2' && \
     echo 'hosts: files mdns4_minimal [NOTFOUND=return] dns mdns4' >> /etc/nsswitch.conf

ENTRYPOINT ["/usr/bin/docker-entrypoint.sh"]

VOLUME ["/data"]

CMD ["minio"]
