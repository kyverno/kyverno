FROM golang:alpine
ADD kyverno /kyverno
RUN apk add --no-cache ca-certificates
USER 10001
ENTRYPOINT ["/kyverno"]