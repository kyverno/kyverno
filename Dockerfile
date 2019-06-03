FROM alpine:latest
WORKDIR ~/
ADD kyverno ./kyverno
ENTRYPOINT ["./kyverno"]
